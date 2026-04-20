package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

var (
	subtle    = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Text().Hex))
	highlight = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Lavender().Hex)).Bold(true)
	success   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Green().Hex)).Bold(true)
	failure   = lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Red().Hex)).Bold(true)
)

const mainTemplate = `package main

import "github.com/zoenetic/orc"

type Plans struct{}

func main() { orc.Main[Plans]() }

func (p *Plans) Default() *orc.Runbook {
	rb := orc.New("hello world", orc.Options{
		Concurrency: 2,
	})

	sayHello := rb.Task("say hello",
		orc.Do{
			Cmds: orc.Sh("echo hello"),
		},
	)

	sayBonjour := rb.Task("say bonjour",
		orc.Do{
			Cmds: orc.Sh("echo bonjour"),
		},
	)

	_ = rb.Task("say goodbye",
		orc.Do{
			Cmds: orc.Sh("echo goodbye"),
		},
		orc.DependsOn{sayHello, sayBonjour},
	)

	return rb
}
`

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new Orc project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		var (
			dir       string
			abs       string
			git       bool
			yes       bool
			confirmed bool
		)

		if len(args) == 1 {
			dir = args[0]
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}

		git, _ = cmd.Flags().GetBool("git")
		yes, _ = cmd.Flags().GetBool("yes")

		if yes {
			confirmed = true
			return runInit(abs, git)
		}

		dirGroup := huh.NewGroup(
			huh.NewInput().
				Title("Project directory").
				Placeholder("new-orc-project").
				Validate(func(input string) error {
					if input == "" {
						return fmt.Errorf("directory cannot be empty")
					}
					return nil
				}).
				Value(&dir),
		).WithHideFunc(func() bool { return dir != "" })

		gitGroup := huh.NewGroup(
			huh.NewConfirm().
				Title("Initialize a git repository?").
				Affirmative("Yes").
				Negative("No").
				Value(&git),
		).WithHideFunc(func() bool {
			return cmd.Flags().Changed("git") || isGitRepo(abs)
		})

		confirmGroup := huh.NewGroup(
			huh.NewConfirm().
				Title("Create project?").
				DescriptionFunc(func() string {
					a, _ := filepath.Abs(dir)
					return subtle.Render(fmt.Sprintf("Will create in %s", highlight.Render(a)))
				}, &dir).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		).WithHideFunc(func() bool { return yes })

		form := huh.NewForm(dirGroup, gitGroup, confirmGroup)

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println(subtle.Render("Cancelled"))
				return nil
			}
			return fmt.Errorf("form: %w", err)
		}

		if !confirmed {
			fmt.Println(subtle.Render("Cancelled"))
			return nil
		}

		abs, err = filepath.Abs(dir)
		if err != nil {
			return err
		}

		return runInit(abs, git)
	},
}

func init() {
	initCmd.Flags().BoolP("git", "g", false, "Initialize a git repository")
	initCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(initCmd)
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func runInit(dir string, git bool) error {
	var (
		sum     *initSummary
		initErr error
	)
	err := spinner.New().
		Title("Initializing project...").
		Action(func() {
			sum, initErr = initProject(dir, git)
		}).
		Run()
	if err != nil {
		return fmt.Errorf("spinner: %w", err)
	}
	if initErr != nil {
		fmt.Println(failure.Render("✗ Failed: " + initErr.Error()))
		return nil
	}

	fmt.Println()
	fmt.Println(success.Render("✓ Project initialized!"))
	fmt.Println()
	fmt.Printf("  %s  %s\n", subtle.Render("dir"), highlight.Render(dir))
	fmt.Println()
	for _, s := range sum.creates {
		fmt.Printf("  %s  %s\n", success.Render("✓"), s)
	}
	for _, s := range sum.skips {
		fmt.Printf("  %s  %s\n", subtle.Render("—"), subtle.Render(s))
	}
	fmt.Println()
	fmt.Println(subtle.Render("Next: cd " + dir + " && go run ."))
	return nil
}

type initSummary struct {
	creates []string
	skips   []string
}

func (s *initSummary) did(label string)     { s.creates = append(s.creates, label) }
func (s *initSummary) skipped(label string) { s.skips = append(s.skips, label) }

func initProject(dir string, git bool) (*initSummary, error) {
	sum := &initSummary{}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	if exists(filepath.Join(dir, "go.mod")) {
		sum.skipped("go.mod (already exists)")
	} else {
		modInit := exec.Command("go", "mod", "init", filepath.Base(dir))
		modInit.Dir = dir
		if out, err := modInit.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("go mod init: %s: %w", out, err)
		}
		goGet := exec.Command("go", "get", "github.com/zoenetic/orc")
		goGet.Dir = dir
		if out, err := goGet.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("go get orc: %s: %w", out, err)
		}
		sum.did("go.mod")
	}

	if exists(filepath.Join(dir, "main.go")) {
		sum.skipped("main.go (already exists)")
	} else {
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainTemplate), 0644); err != nil {
			return nil, fmt.Errorf("write main.go: %w", err)
		}
		sum.did("main.go")
	}

	gitignoreEntities := []string{".orc/", "go.work", "go.work.sum"}
	if added := appendMissing(filepath.Join(dir, ".gitignore"), gitignoreEntities); len(added) > 0 {
		sum.did(fmt.Sprintf(".gitignore (+%s)", strings.Join(added, ", ")))
	} else {
		sum.skipped(".gitignore (already contains orc entries)")
	}

	if git {
		if !isGitRepo(dir) {
			gitInit := exec.Command("git", "init")
			gitInit.Dir = dir
			out, err := gitInit.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("git init: %s: %w", out, err)
			}
			sum.did(fmt.Sprintf("git init: %s", out))
		} else {
			sum.skipped("git repository (already exists)")
		}
	}

	return sum, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func appendMissing(path string, want []string) []string {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil
	}
	existing := string(data)

	var toAdd []string
	for _, entry := range want {
		if !strings.Contains(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}
	defer f.Close()

	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		f.WriteString("\n")
	}
	for _, entry := range toAdd {
		f.WriteString(entry + "\n")
	}
	return toAdd
}
