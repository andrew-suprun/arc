package filesys

import (
	"arc/fs"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/text/unicode/norm"
)

const hashFileName = ".meta.csv"

func (s *fsys) scanArchive(scan scan) {
	defer func() {
		s.events <- fs.ArchiveHashed{
			Root: scan.root,
		}
	}()

	fsys := os.DirFS(scan.root)
	iofs.WalkDir(fsys, ".", func(path string, d iofs.DirEntry, err error) error {
		if s.quit.Load() || !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		if err != nil {
			s.events <- fs.Error{Path: scan.root, Error: err}
			return nil
		}

		meta, err := d.Info()
		if err != nil {
			s.events <- fs.Error{Path: scan.root, Error: err}
			return nil
		}
		sys := meta.Sys().(*syscall.Stat_t)
		modTime := meta.ModTime()
		modTime = modTime.UTC().Round(time.Second)

		file := &fs.FileMeta{
			Root:    scan.root,
			Path:    norm.NFC.String(path),
			Size:    int(meta.Size()),
			ModTime: modTime,
		}

		s.metas[sys.Ino] = file

		return nil
	})

	s.readMeta(scan.root)

	metas := fs.FileMetas{}
	for _, meta := range s.metas {
		metas = append(metas, *meta)
	}
	s.events <- metas

	defer func() {
		s.storeMeta(scan.root)
	}()

	for _, meta := range s.metas {
		if meta.Hash != "" {
			continue
		}
		if s.quit.Load() {
			return
		}
		meta.Hash = s.hashFile(scan.root, meta.Path)
		s.events <- fs.FileHashed{
			Root: scan.root,
			Path: meta.Path,
			Hash: meta.Hash,
		}
	}
}

func (s *fsys) readMeta(root string) {
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

			info, ok := s.metas[iNode]
			if hash != "" && ok && info.ModTime == modTime && info.Size == int(size) {
				s.metas[iNode].Hash = hash
			}
		}
	}
}

func (s *fsys) storeMeta(root string) error {
	result := make([][]string, 1, len(s.metas)+1)
	result[0] = []string{"INode", "Name", "Size", "ModTime", "Hash"}

	for iNode, file := range s.metas {
		if file.Hash == "" {
			continue
		}
		result = append(result, []string{
			fmt.Sprint(iNode),
			norm.NFC.String(file.Path),
			fmt.Sprint(file.Size),
			file.ModTime.UTC().Format(time.RFC3339Nano),
			file.Hash,
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
		if s.quit.Load() {
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
