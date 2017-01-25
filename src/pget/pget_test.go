package pget

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownload(t *testing.T) {
	d := NewDownload("", "", "", 1, 1, false, 0)
	assert.NotEqual(t, d, nil)
}

func TestNewDownload_genBatch(t *testing.T) {
	d := NewDownload("", "", "", 1, 2, false, 0)
	d.size = 10
	d.genBatch()
	assert.Equal(t, len(d.batchMap), 5)
}

func TestNewDownload_genRange(t *testing.T) {
	d := NewDownload("", "", "", 1, 2, false, 0)
	d.size = 10
	start, end := d.genRange(5)
	assert.Equal(t, start, int64(10))
	assert.Equal(t, end, int64(9))
}

func TestNewDownload_genBatch2(t *testing.T) {
	d := NewDownload("", "", "", 1, 3, false, 0)
	d.size = 100
	d.genBatch()
	assert.Equal(t, len(d.batchMap), 34)
}

func TestNewDownload_genRange2(t *testing.T) {
	d := NewDownload("", "", "", 1, 3, false, 0)
	d.size = 100
	start, end := d.genRange(33)
	assert.Equal(t, start, int64(99))
	assert.Equal(t, end, int64(99))
}
