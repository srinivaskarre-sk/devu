package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(gwtCmd)
	gwtCmd.AddCommand(gwtListCmd)
	gwtCmd.AddCommand(gwtCreateCmd)
	gwtCmd.AddCommand(gwtDeleteCmd)
	// short aliases at root level
	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtl",
		Short: "List worktrees (→ gwt list)",
		RunE:  runGwtList,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtc <name>",
		Short: "Create a worktree (→ gwt create)",
		Args:  cobra.ExactArgs(1),
		RunE:  runGwtCreate,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtd",
		Short: "Delete current worktree (→ gwt delete)",
		RunE:  runGwtDelete,
	})
}

var gwtCmd = &cobra.Command{
	Use:   "gwt",
	Short: "Git worktree commands",
}

var gwtListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees for the current repo",
	RunE:  runGwtList,
}

var gwtCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a worktree for <name>",
	Args:  cobra.ExactArgs(1),
	RunE:  runGwtCreate,
}

var gwtDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the current worktree (checks for uncommitted/unpushed changes)",
	RunE:  runGwtDelete,
}

// ── helpers ───────────────────────────────────────────────────────────────────

// getMainRoot returns the main worktree root, works from inside any worktree.
func getMainRoot() (string, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repo")
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree "), nil
		}
	}
	return "", fmt.Errorf("not in a git repo")
}

// worktreesDir returns the directory where worktrees live.
// Prefers the DEVU_WORKTREES_DIR env var, falls back to <mainRoot>/.worktrees.
func worktreesDir(mainRoot string) string {
	if dir := os.Getenv("DEVU_WORKTREES_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(mainRoot, ".worktrees")
}

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	return strings.TrimSpace(line), err
}

// ── subcommands ───────────────────────────────────────────────────────────────

func runGwtList(_ *cobra.Command, _ []string) error {
	mainRoot, err := getMainRoot()
	if err != nil {
		return err
	}

	out, err := exec.Command("git", "worktree", "list").Output()
	if err != nil {
		return err
	}

	dir := worktreesDir(mainRoot)
	fmt.Printf("Repo: %s  (worktrees: %s)\n", filepath.Base(mainRoot), dir)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			name := filepath.Base(parts[0])
			fmt.Printf("  %-28s %s  %s\n", name, parts[1], parts[2])
		}
	}
	return nil
}

func runGwtCreate(_ *cobra.Command, args []string) error {
	name := args[0]
	mainRoot, err := getMainRoot()
	if err != nil {
		return err
	}

	dir := worktreesDir(mainRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create worktrees dir: %w", err)
	}

	path := filepath.Join(dir, name)
	branch := name

	if out, err := exec.Command("git", "worktree", "add", path, "-b", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("created: %s  (branch: %s)\n", path, branch)
	return nil
}

func runGwtDelete(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	mainRoot, err := getMainRoot()
	if err != nil {
		return err
	}
	if filepath.Clean(cwd) == filepath.Clean(mainRoot) {
		return fmt.Errorf("you are in the main worktree — cd into a branch worktree first")
	}

	branchOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("could not determine current branch")
	}
	branch := strings.TrimSpace(string(branchOut))

	statusOut, _ := exec.Command("git", "status", "--porcelain").Output()
	hasUncommitted := strings.TrimSpace(string(statusOut)) != ""

	unpushedOut, _ := exec.Command("git", "log", "@{u}..", "--oneline").Output()
	hasUnpushed := strings.TrimSpace(string(unpushedOut)) != ""

	force := false
	if hasUncommitted || hasUnpushed {
		fmt.Fprintf(os.Stderr, "Warning: '%s' has:\n", branch)
		if hasUncommitted {
			fmt.Fprintf(os.Stderr, "  - uncommitted changes\n")
		}
		if hasUnpushed {
			fmt.Fprintf(os.Stderr, "  - unpushed commits\n")
		}
		input, _ := readLine("Re-enter branch name to confirm deletion: ")
		if input != branch {
			fmt.Fprintln(os.Stderr, "Aborted: branch name did not match.")
			return nil
		}
		force = true
	} else {
		input, _ := readLine(fmt.Sprintf("Delete worktree '%s'? [y/N] ", branch))
		if strings.ToLower(input) != "y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	removeArgs := []string{"worktree", "remove", cwd}
	if force {
		removeArgs = append(removeArgs, "--force")
	}
	if out, err := exec.Command("git", removeArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("deleted: %s\n", cwd)
	return nil
}

