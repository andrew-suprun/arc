package filesys

import (
	"arc/fs"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/text/unicode/norm"
)

const hashFileName = ".meta.csv"

type meta struct {
	inode uint64
	file  *fs.FileMeta
}

func (s *fsys) scanArchive(scan scan) {
	defer func() {
		s.events <- fs.ArchiveHashed{
			Root: scan.root,
		}
	}()

	metaSlice := []*meta{}
	metaMap := map[uint64]*fs.FileMeta{}
	fsys := os.DirFS(scan.root)
	iofs.WalkDir(fsys, ".", func(path string, d iofs.DirEntry, err error) error {
		if s.commands.Closed() || !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		if err != nil {
			s.events <- fs.Error{Path: scan.root, Error: err}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			s.events <- fs.Error{Path: scan.root, Error: err}
			return nil
		}
		sys := info.Sys().(*syscall.Stat_t)
		modTime := info.ModTime()
		modTime = modTime.UTC().Round(time.Second)

		file := &fs.FileMeta{
			Root:    scan.root,
			Path:    norm.NFC.String(path),
			Size:    int(info.Size()),
			ModTime: modTime,
		}

		metaSlice = append(metaSlice, &meta{
			inode: sys.Ino,
			file:  file,
		})
		metaMap[sys.Ino] = file

		return nil
	})

	s.readMeta(scan.root, metaMap)

	slices.SortFunc(metaSlice, func(a, b *meta) int {
		return cmp.Compare(strings.ToLower(a.file.Path), strings.ToLower(b.file.Path))
	})

	slice := make(fs.FileMetas, 0, len(metaSlice))
	for _, meta := range metaSlice {
		slice = append(slice, *meta.file)
	}

	s.events <- slice

	defer func() {
		s.storeMeta(scan.root, metaSlice)
	}()

	for _, meta := range metaSlice {
		if meta.file.Hash != "" {
			continue
		}
		if s.commands.Closed() {
			return
		}
		meta.file.Hash = s.hashFile(scan.root, meta.file.Path)
		s.events <- fs.FileHashed{
			Root: scan.root,
			Path: meta.file.Path,
			Hash: meta.file.Hash,
		}
	}
}

func (s *fsys) readMeta(root string, metas map[uint64]*fs.FileMeta) {
	absHashFileName := filepath.Join(root, hashFileName)
	hashInfoFile, err := os.Open(absHashFileName)
	if err != nil {
		return
	}
	defer hashInfoFile.Close()

	records, err := csv.NewReader(hashInfoFile).ReadAll()
	if err != nil || len(records) == 0 {
		return
	}

	for _, record := range records[1:] {
		if len(record) == 5 {
			iNode, er1 := strconv.ParseUint(record[0], 10, 64)
			size, er2 := strconv.ParseUint(record[2], 10, 64)
			modTime, er3 := time.Parse(time.RFC3339, record[3])
			modTime = modTime.UTC().Round(time.Second)
			hash := record[4]
			if hash == "" || er1 != nil || er2 != nil || er3 != nil {
				continue
			}

			info, ok := metas[iNode]
			if hash != "" && ok && info.ModTime == modTime && info.Size == int(size) {
				metas[iNode].Hash = hash
			}
		}
	}
}

func (s *fsys) storeMeta(root string, metas []*meta) error {
	result := make([][]string, 1, len(metas)+1)
	result[0] = []string{"INode", "Name", "Size", "ModTime", "Hash"}

	for _, meta := range metas {
		if meta.file.Hash == "" {
			continue
		}
		result = append(result, []string{
			fmt.Sprint(meta.inode),
			norm.NFC.String(meta.file.Path),
			fmt.Sprint(meta.file.Size),
			meta.file.ModTime.UTC().Format(time.RFC3339Nano),
			meta.file.Hash,
		})
	}

	absHashFileName := filepath.Join(root, hashFileName)
	hashInfoFile, err := os.Create(absHashFileName)

	if err != nil {
		return err
	}
	err = csv.NewWriter(hashInfoFile).WriteAll(result)
	hashInfoFile.Close()
	return err
}

func (s *fsys) hashFile(root, path string) string {
	hash := sha256.New()
	buf := make([]byte, 1024*1024)
	hashed := 0

	fsys := os.DirFS(root)
	file, err := fsys.Open(path)
	if err != nil {
		s.events <- fs.Error{Path: filepath.Join(root, path), Error: err}
		return ""
	}
	defer file.Close()

	for {
		if s.commands.Closed() {
			return ""
		}

		nr, er := file.Read(buf)
		if nr > 0 {
			nw, ew := hash.Write(buf[0:nr])
			if ew != nil {
				if err != nil {
					s.events <- fs.Error{Path: filepath.Join(root, path), Error: err}
					return ""
				}
			}
			if nr != nw {
				s.events <- fs.Error{Path: filepath.Join(root, path), Error: err}
				return ""
			}
		}

		if er == io.EOF {
			break
		}
		if er != nil {
			s.events <- fs.Error{Path: filepath.Join(root, path), Error: err}
			return ""
		}

		hashed += nr
		s.events <- fs.Progress{
			Root:     root,
			Path:     path,
			Progress: hashed,
		}
	}
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}
