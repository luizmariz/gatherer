package ui

import (
	"fmt"
	"io"
	"time"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner shows an animated line while a command runs. On a non-TTY it prints a
// single static line at Start and the final line at Stop (no animation).
type Spinner struct {
	w     io.Writer
	theme Theme
	msg   string

	stop chan struct{}
	done chan struct{}
}

// Spinner builds a spinner that writes to w with this theme.
func (t Theme) Spinner(w io.Writer, msg string) *Spinner {
	return &Spinner{w: w, theme: t, msg: msg}
}

// Start begins the animation (TTY) or prints the message once (non-TTY).
func (s *Spinner) Start() {
	if !s.theme.TTY {
		fmt.Fprintf(s.w, "  %s %s\n", IconCast, s.msg)
		return
	}

	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	go func() {
		t := time.NewTicker(90 * time.Millisecond)
		defer t.Stop()

		for i := 0; ; i++ {
			select {
			case <-s.stop:
				close(s.done)
				return
			case <-t.C:
				frame := s.theme.paint(spinFrames[i%len(spinFrames)], cyan)
				fmt.Fprintf(s.w, "\r  %s %s", frame, s.msg)
			}
		}
	}()
}

// Stop ends the animation and prints the final status line in its place.
func (s *Spinner) Stop(final string) {
	if !s.theme.TTY {
		fmt.Fprintf(s.w, "  %s\n", final)
		return
	}

	close(s.stop)
	<-s.done
	fmt.Fprintf(s.w, "\r\033[K  %s\n", final)
}
