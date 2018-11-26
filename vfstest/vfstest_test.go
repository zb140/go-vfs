package vfstest

import (
	"log"
	"os"
	"testing"

	"github.com/twpayne/go-vfs"
)

func TestBuilderBuild(t *testing.T) {
	for _, tc := range []struct {
		name  string
		umask os.FileMode
		root  interface{}
		tests interface{}
	}{
		{
			name:  "empty",
			umask: 022,
			tests: []Test{},
		},
		{
			name:  "dir",
			umask: 022,
			root: map[string]interface{}{
				"foo": &Dir{
					Perm: 0755,
					Entries: map[string]interface{}{
						"bar": "baz",
					},
				},
			},
			tests: []Test{
				TestPath("/foo", TestIsDir, TestModePerm(0755)),
				TestPath("/foo/bar", TestModeIsRegular, TestModePerm(0644), TestContentsString("baz")),
			},
		},
		{
			name:  "map_string_string",
			umask: 022,
			root: map[string]string{
				"foo": "bar",
			},
			tests: []Test{
				TestPath("/foo", TestModeIsRegular, TestModePerm(0644), TestContentsString("bar")),
			},
		},
		{
			name:  "map_string_empty_interface",
			umask: 022,
			root: map[string]interface{}{
				"foo": "bar",
				"baz": &File{Perm: 0755, Contents: []byte("qux")},
				"dir": &Dir{Perm: 0700},
			},
			tests: []Test{
				TestPath("/foo", TestModeIsRegular, TestModePerm(0644), TestSize(3), TestContentsString("bar")),
				TestPath("/baz", TestModeIsRegular, TestModePerm(0755), TestSize(3), TestContentsString("qux")),
				TestPath("/dir", TestIsDir, TestModePerm(0700)),
			},
		},
		{
			name:  "long_paths",
			umask: 022,
			root: map[string]string{
				"/foo/bar": "baz",
			},
			tests: []Test{
				TestPath("/foo", TestIsDir, TestModePerm(0755)),
				TestPath("/foo/bar", TestModeIsRegular, TestModePerm(0644), TestSize(3), TestContentsString("baz")),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, cleanup, err := NewTempFS(tc.root, BuilderUmask(tc.umask), BuilderVerbose(true))
			defer cleanup()
			if err != nil {
				t.Fatal(err)
			}
			RunTest(t, fs, "", tc.tests)
		})
	}
}

// TestCoverage exercises as much functionality as possible to increase test
// coverage.
func TestCoverage(t *testing.T) {
	fs, cleanup, err := NewTempFS(map[string]interface{}{
		"/home/user/.bashrc": "# contents of user's .bashrc\n",
		"/home/user/empty":   []byte{},
		"/home/user/bin/hello.sh": &File{
			Perm:     0755,
			Contents: []byte("echo hello\n"),
		},
		"/home/user/foo": map[string]interface{}{
			"bar": map[string]interface{}{
				"baz": "qux",
			},
		},
		"/root": &Dir{
			Perm: 0700,
			Entries: map[string]interface{}{
				".bashrc": "# contents of root's .bashrc\n",
			},
		},
	})
	defer cleanup()
	if err != nil {
		log.Fatal(err)
	}
	RunTest(t, fs, "", []interface{}{
		TestPath("/home",
			TestIsDir,
			TestModePerm(0755)),
		TestPath("/notexist",
			TestDoesNotExist),
		map[string]Test{
			"home_user_bashrc": TestPath("/home/user/.bashrc",
				TestModeIsRegular,
				TestModePerm(0644),
				TestContentsString("# contents of user's .bashrc\n"),
				TestMinSize(1),
				TestSysNlink(1)),
		},
		map[string]interface{}{
			"home_user_empty": TestPath("/home/user/empty",
				TestModeIsRegular,
				TestModePerm(0644),
				TestSize(0)),
			"foo_bar_baz": []Test{
				TestPath("/home/user/foo/bar/baz",
					TestModeIsRegular,
					TestModePerm(0644),
					TestContentsString("qux")),
			},
			"root": []interface{}{
				TestPath("/root",
					TestIsDir,
					TestModePerm(0700)),
				TestPath("/root/.bashrc",
					TestModeIsRegular,
					TestModePerm(0644),
					TestContentsString("# contents of root's .bashrc\n")),
			},
		},
	})
}

func TestErrors(t *testing.T) {
	for name, f := range map[string]func(*Builder, vfs.FS) error{
		"write_file_with_different_content": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user/.bashrc", nil, 0644)
		},
		"write_file_with_different_perms": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user/.bashrc", []byte("# bashrc\n"), 0755)
		},
		"write_file_to_existing_dir": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user", nil, 0644)
		},
		"write_file_via_existing_dir": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user/empty/foo", nil, 0644)
		},
		"mkdir_existing_dir_with_different_perms": func(b *Builder, fs vfs.FS) error {
			return b.Mkdir(fs, "/home/user", 0666)
		},
		"mkdir_to_existing_file": func(b *Builder, fs vfs.FS) error {
			return b.Mkdir(fs, "/home/user/empty", 0755)
		},
		"mkdir_all_to_existing_file": func(b *Builder, fs vfs.FS) error {
			return b.Mkdir(fs, "/home/user/empty", 0755)
		},
		"mkdir_all_via_existing_file": func(b *Builder, fs vfs.FS) error {
			return b.MkdirAll(fs, "/home/user/empty/foo", 0755)
		},
	} {
		t.Run(name, func(t *testing.T) {
			fs, cleanup, err := newTempFS()
			defer cleanup()
			if err != nil {
				t.Fatal(err)
			}
			b := NewBuilder(BuilderVerbose(true))
			root := map[string]interface{}{
				"/home/user/.bashrc": "# bashrc\n",
				"/home/user/empty":   []byte{},
				"/home/user/foo":     &Dir{Perm: 0755},
			}
			if err := b.Build(fs, root); err != nil {
				t.Fatalf("b.Build(fs, root) == %v, want <nil>", err)
			}
			if err := f(b, fs); err == nil {
				t.Error("got <nil>, want !<nil>")
			}
		})
	}
}

func TestIdempotency(t *testing.T) {
	for name, f := range map[string]func(*Builder, vfs.FS) error{
		"write_new_file": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user/empty", nil, 0644)
		},
		"write_file_with_same_content_and_perms": func(b *Builder, fs vfs.FS) error {
			return b.WriteFile(fs, "/home/user/.bashrc", []byte("# bashrc\n"), 0644)
		},
		"mkdir_existing_dir_with_same_perms": func(b *Builder, fs vfs.FS) error {
			return b.Mkdir(fs, "/home/user", 0755)
		},
		"mkdir_new_dir": func(b *Builder, fs vfs.FS) error {
			return b.Mkdir(fs, "/home/user/foo", 0755)
		},
		"mkdir_all_existing_dir": func(b *Builder, fs vfs.FS) error {
			return b.MkdirAll(fs, "/home/user", 0755)
		},
		"mkdir_all_new_dir": func(b *Builder, fs vfs.FS) error {
			return b.MkdirAll(fs, "/usr/bin", 0755)
		},
	} {
		t.Run(name, func(t *testing.T) {
			fs, cleanup, err := newTempFS()
			defer cleanup()
			if err != nil {
				t.Fatal(err)
			}
			b := NewBuilder(BuilderVerbose(true))
			root := map[string]string{
				"/home/user/.bashrc": "# bashrc\n",
			}
			if err := b.Build(fs, root); err != nil {
				t.Fatalf("b.Build(fs, root) == %v, want <nil>", err)
			}
			if err := f(b, fs); err != nil {
				t.Errorf("got %v, want <nil>", err)
			}
		})
	}
}