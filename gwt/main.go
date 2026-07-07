package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed .env
var embeddedEnv string

func init() {
	for _, line := range strings.Split(embeddedEnv, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if os.Getenv(parts[0]) == "" {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func main() {
	root := &cobra.Command{
		Use:   "gwt",
		Short: "Multi-repo git worktree commands",
	}
	root.AddCommand(createCmd(), updateCmd(), listCmd(), deleteCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <feature>",
		Short: "Create worktrees for a feature across repos",
		Args:  cobra.ExactArgs(1),
		RunE:  runCreate,
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <feature>",
		Short: "Add more worktrees to an existing feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			feature := sanitize(args[0])
			feature = strings.TrimPrefix(feature, "d_")
			featureDir := filepath.Join(worktreeBase(), "d_"+feature)
			if _, err := os.Stat(featureDir); os.IsNotExist(err) {
				return fmt.Errorf("feature not found: %s", featureDir)
			}
			return runUpsert(feature, featureDir)
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all feature worktree groups",
		RunE:  runList,
	}
}

func deleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <feature>",
		Short: "Remove all worktrees for a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runDelete(c, args, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip dirty checks and confirmation prompt")
	return cmd
}

// ── types ─────────────────────────────────────────────────────────────────────

type repoEntry struct {
	label string
	path  string
	name  string // basename, used as worktree subdir name
}

// ── config ────────────────────────────────────────────────────────────────────

func escalekit() string    { return os.Getenv("DEVU_ESCALEKIT") }
func worktreeBase() string { return os.Getenv("DEVU_WORKTREE_BASE") }

func coreRepos() []repoEntry {
	e := escalekit()
	return []repoEntry{
		{"be", e + "/scalekit", "scalekit"},
		{"fe", e + "/scalekit-web-ui", "scalekit-web-ui"},
		{"auto", e + "/scalekit-automation", "scalekit-automation"},
	}
}

func sdkRepos() []repoEntry {
	e := escalekit()
	return []repoEntry{
		{"sdk-go", e + "/scalekit-sdk-go", "scalekit-sdk-go"},
		{"sdk-java", e + "/scalekit-sdk-java", "scalekit-sdk-java"},
		{"sdk-node", e + "/scalekit-sdk-node", "scalekit-sdk-node"},
		{"sdk-python", e + "/scalekit-sdk-python", "scalekit-sdk-python"},
	}
}

func docsRepo() repoEntry {
	return repoEntry{"docs", escalekit() + "/developer-docs", "developer-docs"}
}

func featuresRepo() repoEntry {
	return repoEntry{"features", escalekit() + "/features", "features"}
}

func allRepos() []repoEntry {
	return append(coreRepos(), sdkRepos()...)
}

// knownRepos includes all repos that can be selected by label.
func knownRepos() []repoEntry {
	return append(allRepos(), docsRepo(), featuresRepo())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sanitize(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
}

func fetchAndPull(repoPath string) {
	fmt.Printf("  fetching %s...\n", filepath.Base(repoPath))
	exec.Command("git", "-C", repoPath, "fetch", "origin").Run()
	exec.Command("git", "-C", repoPath, "pull", "origin", "main").Run()
}

func detectBranch(featureDir string) string {
	entries, _ := os.ReadDir(featureDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wtPath := filepath.Join(featureDir, e.Name())
		out, err := exec.Command("git", "-C", wtPath, "branch", "--show-current").Output()
		if err == nil {
			if b := strings.TrimSpace(string(out)); b != "" {
				return b
			}
		}
	}
	return ""
}

func copyClaudeMd(featureDir string) {
	src := filepath.Join(escalekit(), "CLAUDE.md")
	dst := filepath.Join(featureDir, "CLAUDE.md")
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not read %s: %v\n", src, err)
		return
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not write CLAUDE.md: %v\n", err)
		return
	}
	fmt.Printf("  CLAUDE.md  →  %s\n", dst)
}

func printRepoMenu() {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Core repos:")
	fmt.Fprintln(os.Stderr, "    be          scalekit (backend)")
	fmt.Fprintln(os.Stderr, "    fe          scalekit-web-ui (frontend)")
	fmt.Fprintln(os.Stderr, "    auto        scalekit-automation")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  SDKs:")
	fmt.Fprintln(os.Stderr, "    sdk-go      scalekit-sdk-go")
	fmt.Fprintln(os.Stderr, "    sdk-java    scalekit-sdk-java")
	fmt.Fprintln(os.Stderr, "    sdk-node    scalekit-sdk-node")
	fmt.Fprintln(os.Stderr, "    sdk-python  scalekit-sdk-python")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Other:")
	fmt.Fprintln(os.Stderr, "    docs        developer-docs")
	fmt.Fprintln(os.Stderr, "    features    features")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Groups:")
	fmt.Fprintln(os.Stderr, "    all         be + fe + auto + all SDKs")
	fmt.Fprintln(os.Stderr, "    core        be + fe + auto")
	fmt.Fprintln(os.Stderr, "    sdks        sdk-go + sdk-java + sdk-node + sdk-python")
	fmt.Fprintln(os.Stderr, "")
}

func promptRepos() ([]repoEntry, error) {
	printRepoMenu()
	fmt.Fprint(os.Stderr, "  Enter comma-separated labels [all]: ")
	scanner := bufio.NewReader(os.Stdin)
	line, err := scanner.ReadString('\n')
	if err != nil {
		return nil, err
	}
	input := strings.TrimSpace(line)
	if input == "" || strings.ToLower(input) == "all" {
		return allRepos(), nil
	}

	byLabel := map[string]repoEntry{}
	for _, r := range knownRepos() {
		byLabel[r.label] = r
	}

	seen := map[string]bool{}
	var selected []repoEntry
	addRepo := func(r repoEntry) {
		if !seen[r.label] {
			seen[r.label] = true
			selected = append(selected, r)
		}
	}

	for _, part := range strings.Split(input, ",") {
		label := strings.TrimSpace(strings.ToLower(part))
		switch label {
		case "all":
			for _, r := range allRepos() {
				addRepo(r)
			}
		case "core":
			for _, r := range coreRepos() {
				addRepo(r)
			}
		case "sdks":
			for _, r := range sdkRepos() {
				addRepo(r)
			}
		default:
			r, ok := byLabel[label]
			if !ok {
				fmt.Fprintf(os.Stderr, "  unknown repo '%s', skipping\n", label)
				continue
			}
			addRepo(r)
		}
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
	out, err := exec.Command("git", "-C", wtPath, "log", "@{u}..", "--oneline").Output()
	if err == nil {
		return strings.TrimSpace(string(out)) != ""
	}
	out2, err2 := exec.Command("git", "-C", wtPath, "log", "origin/main..", "--oneline").Output()
	if err2 != nil {
		return false
	}
	return strings.TrimSpace(string(out2)) != ""
}

// ── create ────────────────────────────────────────────────────────────────────

func runCreate(_ *cobra.Command, args []string) error {
	feature := sanitize(args[0])
	featureDir := filepath.Join(worktreeBase(), "d_"+feature)

	if _, err := os.Stat(featureDir); err == nil {
		return runUpsert(feature, featureDir)
	}

	selected, err := promptRepos()
	if err != nil {
		return err
	}

	branch := "b_" + feature

	if err := os.MkdirAll(featureDir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", featureDir, err)
	}

	var created []string
	for _, r := range selected {
		fetchAndPull(r.path)
		wtPath := filepath.Join(featureDir, r.name)
		out, err := exec.Command("git", "-C", r.path, "worktree", "add", wtPath, "-b", branch, "main").CombinedOutput()
		if err != nil {
			for _, p := range created {
				exec.Command("git", "worktree", "remove", p, "--force").Run()
			}
			os.RemoveAll(featureDir)
			return fmt.Errorf("failed to create worktree for %s: %s", r.label, strings.TrimSpace(string(out)))
		}
		created = append(created, wtPath)
		fmt.Printf("  %-10s  %s  →  %s\n", r.label, r.name, branch)
	}

	copyClaudeMd(featureDir)
	fmt.Printf("\n%s\n", featureDir)
	return nil
}

func runUpsert(feature, featureDir string) error {
	byName := map[string]repoEntry{}
	for _, r := range knownRepos() {
		byName[r.name] = r
	}

	existing := map[string]bool{}
	subs, _ := os.ReadDir(featureDir)
	for _, s := range subs {
		if s.IsDir() {
			existing[s.Name()] = true
		}
	}

	branch := detectBranch(featureDir)
	if branch == "" {
		branch = "b_" + feature
	}

	fmt.Fprintf(os.Stderr, "\n  Feature 'd_%s' already exists  [branch: %s]\n", feature, branch)
	fmt.Fprintln(os.Stderr, "  Existing worktrees:")
	for _, r := range knownRepos() {
		if existing[r.name] {
			fmt.Fprintf(os.Stderr, "    ✓ %-10s  %s\n", r.label, r.name)
		}
	}
	fmt.Fprintln(os.Stderr, "")

	var available []repoEntry
	for _, r := range knownRepos() {
		if !existing[r.name] {
			available = append(available, r)
		}
	}

	if len(available) == 0 {
		fmt.Fprintln(os.Stderr, "  All known repos are already in this feature.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "  Available to add:")
	for _, r := range available {
		fmt.Fprintf(os.Stderr, "    %-10s  %s\n", r.label, r.name)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprint(os.Stderr, "  Enter comma-separated labels to add [none]: ")

	scanner := bufio.NewReader(os.Stdin)
	line, err := scanner.ReadString('\n')
	if err != nil {
		return err
	}
	input := strings.TrimSpace(line)
	if input == "" || strings.ToLower(input) == "none" {
		fmt.Fprintln(os.Stderr, "  Nothing added.")
		return nil
	}

	availableByLabel := map[string]repoEntry{}
	for _, r := range available {
		availableByLabel[r.label] = r
	}
	allByLabel := map[string]repoEntry{}
	for _, r := range knownRepos() {
		allByLabel[r.label] = r
	}

	seen := map[string]bool{}
	var selected []repoEntry
	addRepo := func(r repoEntry) {
		if !seen[r.label] {
			seen[r.label] = true
			selected = append(selected, r)
		}
	}

	for _, part := range strings.Split(input, ",") {
		label := strings.TrimSpace(strings.ToLower(part))
		switch label {
		case "all":
			for _, r := range available {
				addRepo(r)
			}
		case "core":
			for _, r := range coreRepos() {
				if !existing[r.name] {
					addRepo(r)
				}
			}
		case "sdks":
			for _, r := range sdkRepos() {
				if !existing[r.name] {
					addRepo(r)
				}
			}
		default:
			r, ok := availableByLabel[label]
			if !ok {
				if ar, known := allByLabel[label]; known && existing[ar.name] {
					fmt.Fprintf(os.Stderr, "  '%s' is already in this feature, skipping\n", label)
				} else {
					fmt.Fprintf(os.Stderr, "  unknown repo '%s', skipping\n", label)
				}
				continue
			}
			addRepo(r)
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("no valid repos selected")
	}

	for _, r := range selected {
		fetchAndPull(r.path)
		wtPath := filepath.Join(featureDir, r.name)
		out, err := exec.Command("git", "-C", r.path, "worktree", "add", wtPath, "-b", branch, "main").CombinedOutput()
		if err != nil {
			// Branch may already exist in this repo; try checking it out directly.
			out2, err2 := exec.Command("git", "-C", r.path, "worktree", "add", wtPath, branch).CombinedOutput()
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "  failed to add worktree for %s: %s\n", r.label, strings.TrimSpace(string(out)))
				continue
			}
			_ = out2
		}
		fmt.Printf("  %-10s  %s  →  %s\n", r.label, r.name, branch)
	}

	fmt.Printf("\n%s\n", featureDir)
	return nil
}

// ── list ──────────────────────────────────────────────────────────────────────

func runList(_ *cobra.Command, _ []string) error {
	type entry struct {
		path   string
		branch string
	}
	seen := map[string]bool{}
	var results []entry

	for _, r := range allRepos() {
		out, err := exec.Command("git", "-C", r.path, "worktree", "list").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			wtPath := fields[0]
			branch := strings.Trim(fields[2], "[]")
			if !strings.HasPrefix(wtPath, worktreeBase()) || seen[wtPath] {
				continue
			}
			seen[wtPath] = true
			results = append(results, entry{wtPath, branch})
		}
	}

	if len(results) == 0 {
		fmt.Println("no feature worktrees found")
		return nil
	}

	fmt.Printf("Feature worktrees in %s\n\n", worktreeBase())
	for _, e := range results {
		rel := strings.TrimPrefix(e.path, worktreeBase()+"/")
		fmt.Printf("  %-40s  [%s]\n", rel, e.branch)
	}
	return nil
}

// ── delete ────────────────────────────────────────────────────────────────────

func runDelete(_ *cobra.Command, args []string, force bool) error {
	feature := sanitize(args[0])
	feature = strings.TrimPrefix(feature, "d_")

	featureDir := filepath.Join(worktreeBase(), "d_"+feature)
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
			fmt.Fprintf(os.Stderr, "  error: %s has", s.Name())
			if uncommitted {
				fmt.Fprintf(os.Stderr, " uncommitted changes")
			}
			if unpushed {
				if uncommitted {
					fmt.Fprintf(os.Stderr, " and")
				}
				fmt.Fprintf(os.Stderr, " unpushed commits")
			}
			fmt.Fprintln(os.Stderr)
		}
		worktrees = append(worktrees, wtEntry{r, wtPath, dirty})
	}

	if !force {
		for _, wt := range worktrees {
			if wt.dirty {
				return fmt.Errorf("aborting: clean up all repos before deleting")
			}
		}
	}

	if !force {
		input, _ := readLine(fmt.Sprintf("Delete 'd_%s' and all its worktrees? [y/N] ", feature))
		if strings.ToLower(input) != "y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	for _, wt := range worktrees {
		removeArgs := []string{"-C", wt.r.path, "worktree", "remove", wt.wtPath}
		if force {
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
