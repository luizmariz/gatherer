// Package remote planeswalks: it ships the gatherer binary and a working
// directory to a host over SSH, then resolves a decklist there — setting up an
// environment from scratch, remotely. It shells out to the system ssh/rsync/scp
// so it reuses your SSH config, keys, and Tailscale names (zero Go deps).
package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/luizmariz/gatherer/internal/ui"
)

// Options configure a planeswalk.
type Options struct {
	Host       string // user@host, or a Tailscale name
	RemoteDir  string // staging directory on the remote
	LocalDir   string // local working directory to ship
	DeckPath   string // decklist to resolve (must live inside LocalDir)
	BinaryPath string // gatherer binary to ship
	Scry       bool   // dry-run on the remote instead of resolving
	Out        io.Writer
}

// Planeswalk ships LocalDir + the binary to Host and runs gatherer there.
func Planeswalk(ctx context.Context, o Options) error {
	out := o.Out
	if out == nil {
		out = os.Stdout
	}
	th := ui.For(out)

	relDeck, err := filepath.Rel(o.LocalDir, o.DeckPath)
	if err != nil || strings.HasPrefix(relDeck, "..") {
		return fmt.Errorf("decklist %q must live inside --dir %q", o.DeckPath, o.LocalDir)
	}

	fmt.Fprintf(out, "\n%s %s %s\n", ui.Mana(), th.Title("Planeswalking to"), th.Title(o.Host))

	// 1) Scout the plane: detect the remote OS/arch.
	unameOut, err := runStep(th, out, "scouting the plane (uname)",
		cmdRun(sshCmd(ctx, o.Host, "uname -sm", false)))
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", o.Host, err)
	}

	goos, goarch, err := archFromUname(string(unameOut))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "      %s\n", th.Flavor("the plane is "+goos+"/"+goarch))
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		fmt.Fprintf(out, "      %s\n", th.Warn(fmt.Sprintf(
			"⚠ shipped binary is %s/%s — pass --binary <%s/%s build> if it won't run there",
			runtime.GOOS, runtime.GOARCH, goos, goarch)))
	}

	// 2) Prepare the staging directory.
	if _, err := runStep(th, out, "preparing "+o.RemoteDir,
		cmdRun(sshCmd(ctx, o.Host, "mkdir -p "+shellQuote(o.RemoteDir), false))); err != nil {
		return err
	}

	// 3) Carry your library: sync the working directory.
	if _, err := runStep(th, out, "carrying your library ("+o.LocalDir+")", transferDir(ctx, o)); err != nil {
		return err
	}

	// 4) Carry the binary and ready it.
	if _, err := runStep(th, out, "carrying the binary (scp)",
		cmdRun(exec.CommandContext(ctx, "scp", "-q", "-o", "ConnectTimeout=10", o.BinaryPath, o.Host+":"+o.RemoteDir+"/gatherer"))); err != nil {
		return err
	}
	if _, err := runStep(th, out, "readying the binary (chmod)",
		cmdRun(sshCmd(ctx, o.Host, "chmod +x "+shellQuote(o.RemoteDir+"/gatherer"), false))); err != nil {
		return err
	}

	// 5) Arrive: hand off to the remote turn over an interactive session so its
	// own spinners and colors render locally.
	verb := "resolve"
	if o.Scry {
		verb = "scry"
	}
	fmt.Fprintf(out, "\n%s %s\n", ui.IconReady, th.Title("Arrived — resolving on "+o.Host))

	remoteCmd := fmt.Sprintf("cd %s && ./gatherer %s --deck %s --dir .",
		shellQuote(o.RemoteDir), verb, shellQuote(relDeck))

	ssh := sshCmd(ctx, o.Host, remoteCmd, true)
	ssh.Stdin = os.Stdin
	ssh.Stdout = out
	ssh.Stderr = os.Stderr
	return ssh.Run()
}

// sshCmd builds an ssh invocation with a sane connect timeout; tty requests a
// pseudo-terminal so the remote turn's spinners and colors render locally.
func sshCmd(ctx context.Context, host, remoteCmd string, tty bool) *exec.Cmd {
	args := []string{"-o", "ConnectTimeout=10"}
	if tty {
		args = append(args, "-t")
	}
	args = append(args, host, remoteCmd)
	return exec.CommandContext(ctx, "ssh", args...)
}

// transferDir prefers rsync (incremental, prunes stale files) and falls back to
// a tar-over-ssh pipeline when rsync is unavailable.
func transferDir(ctx context.Context, o Options) func() ([]byte, error) {
	if _, err := exec.LookPath("rsync"); err == nil {
		c := exec.CommandContext(ctx, "rsync", "-az", "--delete", "--exclude", ".git",
			"-e", "ssh -o ConnectTimeout=10",
			o.LocalDir+"/", o.Host+":"+o.RemoteDir+"/")
		return cmdRun(c)
	}

	pipeline := fmt.Sprintf("tar -C %s --exclude=.git -czf - . | ssh -o ConnectTimeout=10 %s %s",
		shellQuote(o.LocalDir), shellQuote(o.Host),
		shellQuote("tar -C "+shellQuote(o.RemoteDir)+" -xzf -"))

	return cmdRun(exec.CommandContext(ctx, "sh", "-c", pipeline))
}

// archFromUname maps `uname -sm` output to Go's GOOS/GOARCH.
func archFromUname(unameSM string) (goos, goarch string, err error) {
	fields := strings.Fields(unameSM)
	if len(fields) < 2 {
		return "", "", fmt.Errorf("unexpected uname output %q", strings.TrimSpace(unameSM))
	}

	goos = strings.ToLower(fields[0])

	switch fields[1] {
	case "x86_64", "amd64":
		goarch = "amd64"
	case "aarch64", "arm64":
		goarch = "arm64"
	case "armv7l", "armv6l":
		goarch = "arm"
	default:
		return "", "", fmt.Errorf("unsupported remote arch %q", fields[1])
	}

	return goos, goarch, nil
}

// runStep shows a spinner for one transport step and surfaces output on failure.
func runStep(th ui.Theme, out io.Writer, label string, run func() ([]byte, error)) ([]byte, error) {
	sp := th.Spinner(out, label)
	sp.Start()

	b, err := run()
	if err != nil {
		sp.Stop(th.Bad(ui.IconCounter + " " + label))
		if o := strings.TrimSpace(string(b)); o != "" {
			return b, fmt.Errorf("%w\n%s", err, indent(o))
		}
		return b, err
	}

	sp.Stop(th.OK(ui.IconResolve + " " + label))
	return b, nil
}

func cmdRun(c *exec.Cmd) func() ([]byte, error) {
	return func() ([]byte, error) {
		var buf bytes.Buffer
		c.Stdout = &buf
		c.Stderr = &buf
		err := c.Run()
		return buf.Bytes(), err
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "      │ " + l
	}
	return strings.Join(lines, "\n")
}
