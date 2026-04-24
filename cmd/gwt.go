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
	gwtCmd.AddCommand(gwtCreateCmd)
	gwtCmd.AddCommand(gwtListCmd)
	gwtCmd.AddCommand(gwtDeleteCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtc <feature>",
		Short: "Create feature worktrees (→ gwt create)",
		Args:  cobra.ExactArgs(1),
		RunE:  runGwtCreate,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtl",
		Short: "List feature worktrees (→ gwt list)",
		RunE:  runGwtList,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "gwtd <feature>",
		Short: "Delete feature worktrees (→ gwt delete)",
		Args:  cobra.ExactArgs(1),
		RunE:  runGwtDelete,
	})
}

var gwtCmd = &cobra.Command{
	Use:   "gwt",
	Short: "Multi-repo git worktree commands",
}

var gwtCreateCmd = &cobra.Command{
	Use:   "create <feature>",
	Short: "Create worktrees for a feature across repos",
	Args:  cobra.ExactArgs(1),
	RunE:  runGwtCreate,
}

var gwtListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature worktree groups",
	RunE:  runGwtList,
}

var gwtDeleteCmd = &cobra.Command{
	Use:   "delete <feature>",
	Short: "Remove all worktrees for a feature",
	Args:  cobra.ExactArgs(1),
	RunE:  runGwtDelete,
}

// ── types ────────────────────────────────────────────────────────────────────

type repoEntry struct {
	label string // "be", "fe", "auto"
	path  string // main repo path
	name  string // basename of path, used as worktree subdir name
}

// ── config helpers ────────────────────────────────────────────────────────────

func allRepos() []repoEntry {
	be := os.Getenv("DEVU_REPO_BE")
	fe := os.Getenv("DEVU_REPO_FE")
	auto := os.Getenv("DEVU_REPO_AUTO")
	return []repoEntry{
		{"be", be, filepath.Base(be)},
		{"fe", fe, filepath.Base(fe)},
		{"auto", auto, filepath.Base(auto)},
	}
}

func worktreeBase() string {
	return os.Getenv("DEVU_WORKTREE_BASE")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sanitize(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
}

// versionedFeature returns `feature` if `d_<feature>` doesn't exist,
// otherwise `feature_v1`, `feature_v2`, etc.
func versionedFeature(base, feature string) string {
	if _, err := os.Stat(filepath.Join(base, "d_"+feature)); os.IsNotExist(err) {
		return feature
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_v%d", feature, i)
		if _, err := os.Stat(filepath.Join(base, "d_"+candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}

func promptRepos(all []repoEntry) ([]repoEntry, error) {
	fmt.Fprintf(os.Stderr, "Repos to include — be, fe, auto, all [all]: ")
	scanner := bufio.NewReader(os.Stdin)
	line, err := scanner.ReadString('\n')
	if err != nil {
		return nil, err
	}
	input := strings.TrimSpace(line)
	if input == "" || strings.ToLower(input) == "all" {
		return all, nil
	}

	byLabel := map[string]repoEntry{}
	for _, r := range all {
		byLabel[r.label] = r
	}

	var selected []repoEntry
	for _, part := range strings.Split(input, ",") {
		label := strings.TrimSpace(strings.ToLower(part))
		r, ok := byLabel[label]
		if !ok {
			fmt.Fprintf(os.Stderr, "  unknown repo '%s', skipping\n", label)
			continue
		}
		selected = append(selected, r)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no valid repos selected")
	}
	return selected, nil
}

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	return strings.TrimSpace(line), err
}

func hasUncommitted(wtPath string) bool {
	out, _ := exec.Command("git", "-C", wtPath, "status", "--porcelain").Output()
	return strings.TrimSpace(string(out)) != ""
}

func hasUnpushed(wtPath string) bool {
	// Try configured upstream first.
	out, err := exec.Command("git", "-C", wtPath, "log", "@{u}..", "--oneline").Output()
	if err == nil {
		return strings.TrimSpace(string(out)) != ""
	}
	// No upstream set — compare against origin/main.
	out2, err2 := exec.Command("git", "-C", wtPath, "log", "origin/main..", "--oneline").Output()
	if err2 != nil {
		return false
	}
	return strings.TrimSpace(string(out2)) != ""
}

// ── create ────────────────────────────────────────────────────────────────────

func runGwtCreate(_ *cobra.Command, args []string) error {
	feature := sanitize(args[0])
	base := worktreeBase()
	if base == "" {
		return fmt.Errorf("DEVU_WORKTREE_BASE not set")
	}

	versioned := versionedFeature(base, feature)
	if versioned != feature {
		fmt.Fprintf(os.Stderr, "  d_%s already exists → using d_%s\n", feature, versioned)
	}

	selected, err := promptRepos(allRepos())
	if err != nil {
		return err
	}

	branch := "b_" + versioned
	featureDir := filepath.Join(base, "d_"+versioned)

	if err := os.MkdirAll(featureDir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", featureDir, err)
	}

	var created []string
	for _, r := range selected {
		if r.path == "" {
			fmt.Fprintf(os.Stderr, "  skipping %s: repo path not configured\n", r.label)
			continue
		}
		wtPath := filepath.Join(featureDir, r.name)
		out, err := exec.Command("git", "-C", r.path, "worktree", "add", wtPath, "-b", branch, "main").CombinedOutput()
		if err != nil {
			// Clean up already-created worktrees.
			for _, p := range created {
				exec.Command("git", "worktree", "remove", p, "--force").Run()
			}
			os.RemoveAll(featureDir)
			return fmt.Errorf("failed to create worktree for %s: %s", r.label, strings.TrimSpace(string(out)))
		}
		created = append(created, wtPath)
		fmt.Printf("  %-6s  %s  →  %s\n", r.label, r.name, branch)
	}

	fmt.Printf("\n%s\n", featureDir)
	return nil
}

// ── list ──────────────────────────────────────────────────────────────────────

func runGwtList(_ *cobra.Command, _ []string) error {
	base := worktreeBase()
	if base == "" {
		return fmt.Errorf("DEVU_WORKTREE_BASE not set")
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", base, err)
	}

	var features []os.DirEntry
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "d_") {
			features = append(features, e)
		}
	}

	if len(features) == 0 {
		fmt.Println("no feature worktrees found")
		return nil
	}

	fmt.Printf("Feature worktrees in %s\n\n", base)
	for _, e := range features {
		featureDir := filepath.Join(base, e.Name())
		subs, _ := os.ReadDir(featureDir)
		var repoNames []string
		for _, s := range subs {
			if s.IsDir() {
				repoNames = append(repoNames, s.Name())
			}
		}
		fmt.Printf("  %-40s  [%s]\n", e.Name(), strings.Join(repoNames, ", "))
	}
	return nil
}

// ── delete ────────────────────────────────────────────────────────────────────

func runGwtDelete(_ *cobra.Command, args []string) error {
	feature := sanitize(args[0])
	feature = strings.TrimPrefix(feature, "d_") // allow typing with or without prefix
	base := worktreeBase()
	if base == "" {
		return fmt.Errorf("DEVU_WORKTREE_BASE not set")
	}

	featureDir := filepath.Join(base, "d_"+feature)
	if _, err := os.Stat(featureDir); os.IsNotExist(err) {
		return fmt.Errorf("feature worktree not found: %s", featureDir)
	}

	byName := map[string]repoEntry{}
	for _, r := range allRepos() {
		byName[r.name] = r
	}

	subs, err := os.ReadDir(featureDir)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", featureDir, err)
	}

	type wtEntry struct {
		r      repoEntry
		wtPath string
		dirty  bool
	}

	var worktrees []wtEntry
	for _, s := range subs {
		if !s.IsDir() {
			continue
		}
		r, ok := byName[s.Name()]
		if !ok {
			continue
		}
		wtPath := filepath.Join(featureDir, s.Name())
		uncommitted := hasUncommitted(wtPath)
		unpushed := hasUnpushed(wtPath)
		dirty := uncommitted || unpushed

		if dirty {
			fmt.Fprintf(os.Stderr, "  warning: %s has", s.Name())
			if uncommitted {
				fmt.Fprintf(os.Stderr, " uncommitted changes")
			}
			if unpushed {
				fmt.Fprintf(os.Stderr, " unpushed commits")
			}
			fmt.Fprintln(os.Stderr)
		}
		worktrees = append(worktrees, wtEntry{r, wtPath, dirty})
	}

	anyDirty := false
	for _, wt := range worktrees {
		if wt.dirty {
			anyDirty = true
			break
		}
	}

	if anyDirty {
		input, _ := readLine(fmt.Sprintf("\nType 'd_%s' to confirm forced deletion: ", feature))
		if input != "d_"+feature {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	} else {
		input, _ := readLine(fmt.Sprintf("Delete 'd_%s' and all its worktrees? [y/N] ", feature))
		if strings.ToLower(input) != "y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	for _, wt := range worktrees {
		removeArgs := []string{"-C", wt.r.path, "worktree", "remove", wt.wtPath}
		if wt.dirty {
			removeArgs = append(removeArgs, "--force")
		}
		if out, err := exec.Command("git", removeArgs...).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not remove worktree %s: %s\n", wt.r.name, strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("  worktree removed: %s\n", wt.r.name)
		}
	}

	if err := os.RemoveAll(featureDir); err != nil {
		return fmt.Errorf("could not remove %s: %w", featureDir, err)
	}
	fmt.Printf("deleted: %s\n", featureDir)
	return nil
}
