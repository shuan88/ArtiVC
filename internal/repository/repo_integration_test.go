// Run integration test to any repository
//
// TEST_REPOSITORY=s3://bucket/myrepo go test -v ./internal/repository
package repository

import (
	cryptorand "crypto/rand"
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getRepo() (Repository, error) {
	repoStr := os.Getenv("TEST_REPOSITORY")
	if repoStr == "" {
		return nil, nil
	}
	return NewRepository(repoStr)
}

func sha1sum(path string) string {
	hasher := sha1.New()
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err := io.Copy(hasher, f); err != nil {
		panic(err)
	}
	sum := hasher.Sum([]byte{})
	return fmt.Sprintf("%x", sum)
}

func generateRandomFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.CopyN(f, cryptorand.Reader, size)
	if err != nil {
		return err
	}

	return nil
}

func Test_Transfer(t *testing.T) {
	repo, err := getRepo()
	if repo == nil {
		return
	}

	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		desc     string
		size     int64
		repoPath string
	}{
		{desc: "small file", size: 1024, repoPath: "bin"},
		{desc: "small file with subpath", size: 1024, repoPath: "this/is/my/bin"},
		{desc: "large file", size: 10 * 1024 * 1024, repoPath: "bin"},
		{desc: "empty file", size: 0, repoPath: "bin"},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := tmpDir + "/in"
			generateRandomFile(path, tC.size)

			err = repo.Upload(path, tC.repoPath, nil)
			if err != nil {
				t.Error(err)
			}

			repo.Download(tC.repoPath, tmpDir+"/out", nil)
			if err != nil {
				t.Error(err)
			}

			assert.Equal(t, sha1sum(tmpDir+"/in"), sha1sum(tmpDir+"/out"))

			err = repo.Delete(tC.repoPath)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
func Test_Stat(t *testing.T) {
	repo, err := getRepo()
	if repo == nil {
		return
	}

	if err != nil {
		t.Error(err)
	}

	rand.Seed(time.Now().UnixNano())
	tmpDir := t.TempDir()
	path := tmpDir + "/bin"
	repoPath := fmt.Sprintf("stat/%d", rand.Int())

	// stat non-existed file
	_, err = repo.Stat(repoPath)
	assert.Error(t, err, "Stat() should return error if the file does not exist")

	// upload & stat
	generateRandomFile(path, 1024)
	err = repo.Upload(path, repoPath, nil)
	if err != nil {
		t.Error(err)
	}

	info, err := repo.Stat(repoPath)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, filepath.Base(repoPath), info.Name(), "name of Stat() should be the last component of path")
	assert.Equal(t, false, info.IsDir(), "result of Stat() should not be a directory ")

	// delete
	err = repo.Delete(repoPath)
	if err != nil {
		t.Error(err)
	}

	_, err = repo.Stat(repoPath)
	assert.Error(t, err, "Stat() should return error after the file deleted")
}

func Test_List(t *testing.T) {
	repo, err := getRepo()
	if repo == nil {
		return
	}

	if err != nil {
		t.Error(err)
	}

	rand.Seed(time.Now().UnixNano())
	tmpDir := t.TempDir()
	path := tmpDir + "/bin"
	generateRandomFile(path, 1024)

	// Create files
	//
	// dir
	// ├── 0
	// ├── 1
	// ├── 2
	// └── 3
	// 	   ├── 0
	// 	   ├── 1
	// 	   └── 2
	for i := 0; i < 3; i++ {
		rpath := fmt.Sprintf("dir/%d", i)
		err = repo.Upload(path, rpath, nil)
		if err != nil {
			t.Error(err)
		}

		defer repo.Delete(rpath)
	}
	for i := 0; i < 3; i++ {
		rpath := fmt.Sprintf("dir/3/%d", i)

		err = repo.Upload(path, rpath, nil)
		if err != nil {
			t.Error(err)
		}

		defer repo.Delete(rpath)
	}

	// test
	// ls dir
	list, err := repo.List("dir")
	assert.Equal(t, 4, len(list))
	for _, info := range list {
		switch info.Name() {
		case "0":
			assert.False(t, info.IsDir())
		case "1":
			assert.False(t, info.IsDir())
		case "2":
			assert.False(t, info.IsDir())
		case "3":
			assert.True(t, info.IsDir())
		default:
			assert.Fail(t, "wrong list item")
		}
	}

	// ls dir/3
	list, err = repo.List("dir/3")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 3, len(list))
	for _, info := range list {
		switch info.Name() {
		case "0":
			assert.False(t, info.IsDir())
		case "1":
			assert.False(t, info.IsDir())
		case "2":
			assert.False(t, info.IsDir())
		default:
			assert.Fail(t, "wrong list item")
		}
	}

	// ls nono-existing folder
	list, err = repo.List("dir-12345")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(list))
}
