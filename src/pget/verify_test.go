package pget

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMD5sum(t *testing.T) {

	f, _ := os.Create("/tmp/test.txt")
	f.WriteString("hello,wrold")
	f.Close()
	md5, _ := MD5sum("/tmp/test.txt")
	assert.Equal(t, md5, "2e9dd21c7bf65eb6c8337e58c658d44c")
}
