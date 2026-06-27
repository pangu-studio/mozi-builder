package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	newModule     string
	newPath       string
	newUIDir      string
	newDesktop    bool
	newMiniapp    bool
	newDesktopDir string
	newMiniappDir string
	newForce      bool
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new mozi-builder-based project",
	Long: `Creates a new project with a mozi-builder-powered backend and an Ant Design
web frontend.  Optionally scaffolds a Tauri desktop client and/or WeChat
mini-program client from minimal skeletons.

The project is a monorepo (大仓): the backend lives at the root level and the
frontends are sibling directories (default: <name>-ui, <name>-desktop,
<name>-miniapp).

After scaffolding, run 'go mod tidy' and 'npm install' in the created
directories to resolve dependencies, then 'mozi init' to set up the design
database.`,

	Example: `  mozi new myapp --module github.com/example/myapp
  mozi new myapp --module github.com/example/myapp --desktop --miniapp
  mozi new myapp --module github.com/example/myapp --ui-dir myapp-admin --force`,

	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().StringVar(&newModule, "module", "", "Go module path (required, e.g. github.com/foo/myapp)")
	newCmd.Flags().StringVar(&newPath, "path", ".", "Parent directory to create the project in")
	newCmd.Flags().StringVar(&newUIDir, "ui-dir", "", "Web frontend directory name (default: <name>-ui)")
	newCmd.Flags().BoolVar(&newDesktop, "desktop", false, "Also scaffold a Tauri v2 desktop client")
	newCmd.Flags().BoolVar(&newMiniapp, "miniapp", false, "Also scaffold a WeChat mini-program (Taro 4)")
	newCmd.Flags().StringVar(&newDesktopDir, "desktop-dir", "", "Desktop directory name (default: <name>-desktop)")
	newCmd.Flags().StringVar(&newMiniappDir, "miniapp-dir", "", "Mini-program directory name (default: <name>-miniapp)")
	newCmd.Flags().BoolVar(&newForce, "force", false, "Overwrite existing non-empty directory")

	_ = newCmd.MarkFlagRequired("module")

	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	if newModule == "" {
		return fmt.Errorf("--module is required (e.g. --module github.com/foo/myapp)")
	}

	// Default frontend dirs
	uiDir := newUIDir
	if uiDir == "" {
		uiDir = name + "-ui"
	}
	desktopDir := newDesktopDir
	if desktopDir == "" {
		desktopDir = name + "-desktop"
	}
	miniappDir := newMiniappDir
	if miniappDir == "" {
		miniappDir = name + "-miniapp"
	}

	// Resolve absolute target path
	parentDir, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("resolve --path: %w", err)
	}
	rootDir := filepath.Join(parentDir, name)

	// Build template variables
	vars := newScaffoldVars(name, newModule, uiDir, desktopDir, miniappDir)
	opts := scaffoldOpts{
		Desktop: newDesktop,
		Miniapp: newMiniapp,
		Force:   newForce,
	}

	fmt.Printf("Creating project %s …\n", rootDir)
	if err := runScaffold(rootDir, vars, opts); err != nil {
		return err
	}

	// Best-effort go mod tidy
	fmt.Println()
	fmt.Println("→ Running go mod tidy …")
	if err := runModTidy(rootDir); err != nil {
		fmt.Printf("  (skipped: %v — run 'go mod tidy' manually)\n", err)
	} else {
		fmt.Println("  ✓ go mod tidy complete")
	}

	// Print next steps
	printNextSteps(rootDir, vars, opts)

	return nil
}

func runModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOPROXY=https://goproxy.cn,direct")
	return cmd.Run()
}

func printNextSteps(rootDir string, vars scaffoldVars, opts scaffoldOpts) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  ✓ Project scaffolded: %s\n", rootDir)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println()
	fmt.Printf("  cd %s\n", filepath.Base(rootDir))
	fmt.Println()
	if !opts.Force {
		// tidy was already attempted; show manual fallback if it likely failed
	}
	fmt.Printf("  # Install UI dependencies & build\n")
	fmt.Printf("  cd %s && npm install && cd ..\n", vars.UIDir)
	fmt.Println()
	if opts.Desktop {
		fmt.Printf("  # Install desktop dependencies\n")
		fmt.Printf("  cd %s && npm install\n", vars.DesktopDir)
		fmt.Printf("  # Then run: npm run tauri dev\n")
		fmt.Println()
	}
	if opts.Miniapp {
		fmt.Printf("  # Install miniapp dependencies\n")
		fmt.Printf("  cd %s && npm install\n", vars.MiniappDir)
		fmt.Printf("  # Then run: npm run dev:weapp\n")
		fmt.Println()
	}
	fmt.Println("  # Initialize design database (PostgreSQL required)")
	fmt.Println("  export MOZI_DB=postgres://localhost:5432/<db>?sslmode=disable")
	fmt.Println("  mozi init")
	fmt.Println()
	fmt.Println("  # Start backend dev server")
	fmt.Printf("  %s=true make dev\n", vars.DevPlatformEnv)
	fmt.Println()
	fmt.Println("  # Or start UI dev server separately:")
	fmt.Printf("  cd %s && npm run dev\n", vars.UIDir)
	fmt.Println()
	fmt.Println("  # Start modeling:")
	fmt.Println("  mozi model create --json '<model-ir>'")
	fmt.Println("  mozi validate")
	fmt.Println("  mozi diff --model <Module/Model>")
	fmt.Println("  mozi change-plan --model <Module/Model>")
}
