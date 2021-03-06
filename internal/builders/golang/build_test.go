package golang

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/goreleaser/goreleaser/internal/artifact"
	"github.com/goreleaser/goreleaser/internal/testlib"
	"github.com/goreleaser/goreleaser/internal/tmpl"
	api "github.com/goreleaser/goreleaser/pkg/build"
	"github.com/goreleaser/goreleaser/pkg/config"
	"github.com/goreleaser/goreleaser/pkg/context"
	"github.com/stretchr/testify/assert"
)

var runtimeTarget = runtime.GOOS + "_" + runtime.GOARCH

func TestWithDefaults(t *testing.T) {
	for name, testcase := range map[string]struct {
		build   config.Build
		targets []string
	}{
		"full": {
			build: config.Build{
				ID:     "foo",
				Binary: "foo",
				Goos: []string{
					"linux",
					"windows",
					"darwin",
				},
				Goarch: []string{
					"amd64",
					"arm",
				},
				Goarm: []string{
					"6",
				},
			},
			targets: []string{
				"linux_amd64",
				"darwin_amd64",
				"windows_amd64",
				"linux_arm_6",
			},
		},
		"empty": {
			build: config.Build{
				ID:     "foo2",
				Binary: "foo",
			},
			targets: []string{
				"linux_amd64",
				"linux_386",
				"darwin_amd64",
				"darwin_386",
			},
		},
	} {
		t.Run(name, func(tt *testing.T) {
			var config = config.Project{
				Builds: []config.Build{
					testcase.build,
				},
			}
			var ctx = context.New(config)
			ctx.Git.CurrentTag = "5.6.7"
			var build = Default.WithDefaults(ctx.Config.Builds[0])
			assert.ElementsMatch(t, build.Targets, testcase.targets)
		})
	}
}

func TestBuild(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				ID:     "foo",
				Env:    []string{"GO111MODULE=off"},
				Binary: "foo",
				Targets: []string{
					"linux_amd64",
					"darwin_amd64",
					"windows_amd64",
					"linux_arm_6",
					"js_wasm",
				},
				Asmflags: []string{".=", "all="},
				Gcflags:  []string{"all="},
				Flags:    []string{"{{.Env.GO_FLAGS}}"},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Env["GO_FLAGS"] = "-v"
	ctx.Git.CurrentTag = "5.6.7"
	var build = ctx.Config.Builds[0]
	for _, target := range build.Targets {
		var ext string
		if strings.HasPrefix(target, "windows") {
			ext = ".exe"
		}
		if target == "js_wasm" {
			ext = ".wasm"
		}
		var err = Default.Build(ctx, build, api.Options{
			Target: target,
			Name:   build.Binary,
			Path:   filepath.Join(folder, "dist", target, build.Binary),
			Ext:    ext,
		})
		assert.NoError(t, err)
	}
	assert.ElementsMatch(t, ctx.Artifacts.List(), []*artifact.Artifact{
		{
			Name:   "foo",
			Path:   filepath.Join(folder, "dist", "linux_amd64", "foo"),
			Goos:   "linux",
			Goarch: "amd64",
			Type:   artifact.Binary,
			Extra: map[string]interface{}{
				"Ext":    "",
				"Binary": "foo",
				"ID":     "foo",
			},
		},
		{
			Name:   "foo",
			Path:   filepath.Join(folder, "dist", "darwin_amd64", "foo"),
			Goos:   "darwin",
			Goarch: "amd64",
			Type:   artifact.Binary,
			Extra: map[string]interface{}{
				"Ext":    "",
				"Binary": "foo",
				"ID":     "foo",
			},
		},
		{
			Name:   "foo",
			Path:   filepath.Join(folder, "dist", "linux_arm_6", "foo"),
			Goos:   "linux",
			Goarch: "arm",
			Goarm:  "6",
			Type:   artifact.Binary,
			Extra: map[string]interface{}{
				"Ext":    "",
				"Binary": "foo",
				"ID":     "foo",
			},
		},
		{
			Name:   "foo",
			Path:   filepath.Join(folder, "dist", "windows_amd64", "foo"),
			Goos:   "windows",
			Goarch: "amd64",
			Type:   artifact.Binary,
			Extra: map[string]interface{}{
				"Ext":    ".exe",
				"Binary": "foo",
				"ID":     "foo",
			},
		},
		{
			Name:   "foo",
			Path:   filepath.Join(folder, "dist", "js_wasm", "foo"),
			Goos:   "js",
			Goarch: "wasm",
			Type:   artifact.Binary,
			Extra: map[string]interface{}{
				"Ext":    ".wasm",
				"Binary": "foo",
				"ID":     "foo",
			},
		},
	})
}

func TestBuildFailed(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				ID:    "buildid",
				Flags: []string{"-flag-that-dont-exists-to-force-failure"},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	var err = Default.Build(ctx, ctx.Config.Builds[0], api.Options{
		Target: "darwin_amd64",
	})
	assertContainsError(t, err, `flag provided but not defined: -flag-that-dont-exists-to-force-failure`)
	assert.Empty(t, ctx.Artifacts.List())
}

func TestBuildInvalidTarget(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var target = "linux"
	var config = config.Project{
		Builds: []config.Build{
			{
				ID:      "foo",
				Binary:  "foo",
				Targets: []string{target},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	var build = ctx.Config.Builds[0]
	var err = Default.Build(ctx, build, api.Options{
		Target: target,
		Name:   build.Binary,
		Path:   filepath.Join(folder, "dist", target, build.Binary),
	})
	assert.EqualError(t, err, "linux is not a valid build target")
	assert.Len(t, ctx.Artifacts.List(), 0)
}

func TestRunInvalidAsmflags(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				Binary:   "nametest",
				Asmflags: []string{"{{.Version}"},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	var err = Default.Build(ctx, ctx.Config.Builds[0], api.Options{
		Target: runtimeTarget,
	})
	assert.EqualError(t, err, `template: tmpl:1: unexpected "}" in operand`)
}

func TestRunInvalidGcflags(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				Binary:  "nametest",
				Gcflags: []string{"{{.Version}"},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	var err = Default.Build(ctx, ctx.Config.Builds[0], api.Options{
		Target: runtimeTarget,
	})
	assert.EqualError(t, err, `template: tmpl:1: unexpected "}" in operand`)
}

func TestRunInvalidLdflags(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				Binary:  "nametest",
				Flags:   []string{"-v"},
				Ldflags: []string{"-s -w -X main.version={{.Version}"},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	var err = Default.Build(ctx, ctx.Config.Builds[0], api.Options{
		Target: runtimeTarget,
	})
	assert.EqualError(t, err, `template: tmpl:1: unexpected "}" in operand`)
}

func TestRunInvalidFlags(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeGoodMain(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				Binary: "nametest",
				Flags:  []string{"{{.Env.GOOS}"},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	var err = Default.Build(ctx, ctx.Config.Builds[0], api.Options{
		Target: runtimeTarget,
	})
	assert.EqualError(t, err, `template: tmpl:1: unexpected "}" in operand`)
}

func TestRunPipeWithoutMainFunc(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	writeMainWithoutMainFunc(t, folder)
	var config = config.Project{
		Builds: []config.Build{
			{
				Binary: "no-main",
				Hooks:  config.Hooks{},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	t.Run("empty", func(t *testing.T) {
		ctx.Config.Builds[0].Main = ""
		assert.EqualError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}), `build for no-main does not contain a main function`)
	})
	t.Run("not main.go", func(t *testing.T) {
		ctx.Config.Builds[0].Main = "foo.go"
		assert.EqualError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}), `stat foo.go: no such file or directory`)
	})
	t.Run("glob", func(t *testing.T) {
		ctx.Config.Builds[0].Main = "."
		assert.EqualError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}), `build for no-main does not contain a main function`)
	})
	t.Run("fixed main.go", func(t *testing.T) {
		ctx.Config.Builds[0].Main = "main.go"
		assert.EqualError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}), `build for no-main does not contain a main function`)
	})
}

func TestRunPipeWithMainFuncNotInMainGoFile(t *testing.T) {
	folder, back := testlib.Mktmp(t)
	defer back()
	assert.NoError(t, ioutil.WriteFile(
		filepath.Join(folder, "foo.go"),
		[]byte("package main\nfunc main() {println(0)}"),
		0644,
	))
	var config = config.Project{
		Builds: []config.Build{
			{
				Env:    []string{"GO111MODULE=off"},
				Binary: "foo",
				Hooks:  config.Hooks{},
				Targets: []string{
					runtimeTarget,
				},
			},
		},
	}
	var ctx = context.New(config)
	ctx.Git.CurrentTag = "5.6.7"
	t.Run("empty", func(t *testing.T) {
		ctx.Config.Builds[0].Main = ""
		assert.NoError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}))
	})
	t.Run("foo.go", func(t *testing.T) {
		ctx.Config.Builds[0].Main = "foo.go"
		assert.NoError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}))
	})
	t.Run("glob", func(t *testing.T) {
		ctx.Config.Builds[0].Main = "."
		assert.NoError(t, Default.Build(ctx, ctx.Config.Builds[0], api.Options{
			Target: runtimeTarget,
		}))
	})
}

func TestLdFlagsFullTemplate(t *testing.T) {
	var ctx = &context.Context{
		Git: context.GitInfo{
			CurrentTag: "v1.2.3",
			Commit:     "123",
		},
		Version: "1.2.3",
		Env:     map[string]string{"FOO": "123"},
	}
	var artifact = &artifact.Artifact{Goarch: "amd64"}
	flags, err := tmpl.New(ctx).WithArtifact(artifact, map[string]string{}).
		Apply(`-s -w -X main.version={{.Version}} -X main.tag={{.Tag}} -X main.date={{.Date}} -X main.commit={{.Commit}} -X "main.foo={{.Env.FOO}}" -X main.time={{ time "20060102" }} -X main.arch={{.Arch}}`)
	assert.NoError(t, err)
	assert.Contains(t, flags, "-s -w")
	assert.Contains(t, flags, "-X main.version=1.2.3")
	assert.Contains(t, flags, "-X main.tag=v1.2.3")
	assert.Contains(t, flags, "-X main.commit=123")
	assert.Contains(t, flags, fmt.Sprintf("-X main.date=%d", time.Now().Year()))
	assert.Contains(t, flags, fmt.Sprintf("-X main.time=%d", time.Now().Year()))
	assert.Contains(t, flags, `-X "main.foo=123"`)
	assert.Contains(t, flags, `-X main.arch=amd64`)
}

func TestInvalidTemplate(t *testing.T) {
	for template, eerr := range map[string]string{
		"{{ .Nope }":    `template: tmpl:1: unexpected "}" in operand`,
		"{{.Env.NOPE}}": `template: tmpl:1:6: executing "tmpl" at <.Env.NOPE>: map has no entry for key "NOPE"`,
	} {
		t.Run(template, func(tt *testing.T) {
			var ctx = context.New(config.Project{})
			ctx.Git.CurrentTag = "3.4.1"
			flags, err := tmpl.New(ctx).Apply(template)
			assert.EqualError(tt, err, eerr)
			assert.Empty(tt, flags)
		})
	}
}

func TestProcessFlags(t *testing.T) {
	var ctx = &context.Context{
		Version: "1.2.3",
	}
	ctx.Git.CurrentTag = "5.6.7"

	var artifact = &artifact.Artifact{
		Name:   "name",
		Goos:   "darwin",
		Goarch: "amd64",
		Goarm:  "7",
		Extra: map[string]interface{}{
			"Binary": "binary",
		},
	}

	var source = []string{
		"flag",
		"{{.Version}}",
		"{{.Os}}",
		"{{.Arch}}",
		"{{.Arm}}",
		"{{.Binary}}",
		"{{.ArtifactName}}",
	}

	var expected = []string{
		"-testflag=flag",
		"-testflag=1.2.3",
		"-testflag=darwin",
		"-testflag=amd64",
		"-testflag=7",
		"-testflag=binary",
		"-testflag=name",
	}

	flags, err := processFlags(ctx, artifact, []string{}, source, "-testflag=")
	assert.NoError(t, err)
	assert.Len(t, flags, 7)
	assert.Equal(t, expected, flags)
}

func TestProcessFlagsInvalid(t *testing.T) {
	var ctx = &context.Context{}

	var source = []string{
		"{{.Version}",
	}

	var expected = `template: tmpl:1: unexpected "}" in operand`

	flags, err := processFlags(ctx, &artifact.Artifact{}, []string{}, source, "-testflag=")
	assert.EqualError(t, err, expected)
	assert.Nil(t, flags)
}

func TestJoinLdFlags(t *testing.T) {
	tests := []struct {
		input  []string
		output string
	}{
		{[]string{"-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=goreleaser"}, "-ldflags=-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=goreleaser"},
		{[]string{"-s -w", "-X main.version={{.Version}}"}, "-ldflags=-s -w -X main.version={{.Version}}"},
	}

	for _, test := range tests {
		joinedLdFlags := joinLdFlags(test.input)
		assert.Equal(t, joinedLdFlags, test.output)
	}
}

//
// Helpers
//

func writeMainWithoutMainFunc(t *testing.T, folder string) {
	assert.NoError(t, ioutil.WriteFile(
		filepath.Join(folder, "main.go"),
		[]byte("package main\nconst a = 2\nfunc notMain() {println(0)}"),
		0644,
	))
}

func writeGoodMain(t *testing.T, folder string) {
	assert.NoError(t, ioutil.WriteFile(
		filepath.Join(folder, "main.go"),
		[]byte("package main\nvar a = 1\nfunc main() {println(0)}"),
		0644,
	))
}

func assertContainsError(t *testing.T, err error, s string) {
	assert.Error(t, err)
	assert.Contains(t, err.Error(), s)
}
