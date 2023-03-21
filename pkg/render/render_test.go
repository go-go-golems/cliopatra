package render

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestComputeBaseDirectory(t *testing.T) {
	s := ComputeBaseDirectory("test.txt", []string{}, "")
	assert.Equal(t, ".", s)

	s = ComputeBaseDirectory("foobar/test.txt", []string{}, "")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{}, "")
	assert.Equal(t, "foobar/foo", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/"}, "")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/foo/", "foo/"}, "")
	assert.Equal(t, "foobar/foo", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/foo/", "foobar/", "foo/"}, "")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/foo/test.txt", "foobar/", "foo/"}, "")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/foo/test.txt"}, "")
	assert.Equal(t, "foobar/foo", s)
}

func TestComputeBaseDirectoryWithBase(t *testing.T) {
	s := ComputeBaseDirectory("test.txt", []string{}, "foobar")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/test.txt", []string{}, "foobar")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{}, "foobar")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/"}, "foobar")
	assert.Equal(t, "foobar", s)

	s = ComputeBaseDirectory("foobar/foo/test.txt", []string{"foobar/foo/", "foo/"}, "foobar")
	assert.Equal(t, "foobar", s)
}
