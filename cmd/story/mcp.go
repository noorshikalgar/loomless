// story mcp: the agent-driven capture path (Phase 1.5, per
// docs/product/09-roadmap.md) — an MCP server exposing the same
// internal/engine that the CLI uses, over stdio, per Scenario B in
// docs/product/04-usage-scenarios.md. An agent (Claude Code, say) calls
// start_recording, narrates its own steps via capture_step/run_command as
// it works, then stop_recording + generate_story to hand back a Markdown
// write-up. Nothing here writes to os.Stdout directly — that's the
// protocol stream; see internal/engine's package comment.
package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noorshikalgar/story/internal/capture"
	"github.com/noorshikalgar/story/internal/engine"
)

func runMCPServer(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:        "story",
		Version:     "0.1.0",
		Description: "Capture a working session as a structured, skimmable story instead of a video.",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "start_recording",
		Description: "Begin a new Story recording session. Also starts automatic " +
			"click capture in the background (best-effort — needs macOS " +
			"Accessibility permission; if unavailable, the session still works " +
			"via capture_step/run_command). Call this once at the start of a " +
			"task you want documented.",
	}, startRecordingTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "capture_step",
		Description: "Capture the current step: a screenshot of the frontmost " +
			"window plus your own note explaining what you did and why. Prefer " +
			"this over letting click-capture alone tell the story — your note " +
			"is what makes the result read like a narrative instead of a flat " +
			"list of clicks. Call it after each meaningful action, not after " +
			"every tiny one: a good granularity is roughly one call per " +
			"reasoning step (e.g. \"found the bug\", \"applied the fix\", not " +
			"one call per file read).",
	}, captureStepTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "run_command",
		Description: "Run a shell command and capture it plus its output as a " +
			"step — use this instead of your own shell tool when you want the " +
			"command to show up in the resulting story (e.g. running a test " +
			"suite to show it passing). The command runs directly, not through " +
			"a shell, so pipes/globs need an explicit [\"sh\", \"-c\", \"...\"]. " +
			"Output is pattern-matched for common secret shapes and masked " +
			"before it's saved, but that's a safety net, not a guarantee — " +
			"avoid commands whose output includes real credentials.",
	}, runCommandTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stop_recording",
		Description: "Finish the active recording session.",
	}, stopRecordingTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "generate_story",
		Description: "Render a session, default format Markdown. Markdown's " +
			"content is returned directly — include it in your final response, " +
			"or write it into the PR/ticket you're producing. PDF is written to " +
			"disk only (content isn't returned inline); use it when the human " +
			"wants a shareable file rather than inline text.",
	}, generateStoryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_windows",
		Description: "List currently open windows (app + title). Useful to " +
			"sanity-check what capture_step/run_command would currently " +
			"resolve as the frontmost window before capturing.",
	}, listWindowsTool)

	return server.Run(ctx, &mcp.StdioTransport{})
}

type startRecordingIn struct {
	Name string `json:"name" jsonschema:"A short, filesystem-safe name for this session, e.g. 'fix-pagination-bug'."`
}

type startRecordingOut struct {
	SessionDir     string `json:"session_dir"`
	WatchingClicks bool   `json:"watching_clicks"`
	Warning        string `json:"warning,omitempty"`
}

func startRecordingTool(_ context.Context, _ *mcp.CallToolRequest, in startRecordingIn) (*mcp.CallToolResult, startRecordingOut, error) {
	if in.Name == "" {
		return nil, startRecordingOut{}, fmt.Errorf("name is required")
	}
	dir, watching, warning, err := engine.StartRecording(in.Name)
	if err != nil {
		return nil, startRecordingOut{}, err
	}
	return nil, startRecordingOut{SessionDir: dir, WatchingClicks: watching, Warning: warning}, nil
}

type captureStepIn struct {
	Note string `json:"note" jsonschema:"What you just did and why — this becomes the step's caption in the rendered story."`
}

type stepOut struct {
	StepIndex   int      `json:"step_index"`
	WindowApp   string   `json:"window_app,omitempty"`
	WindowTitle string   `json:"window_title,omitempty"`
	Masked      []string `json:"masked,omitempty" jsonschema:"Names of secret-masking rules that fired on this step's text, if any."`
	Warning     string   `json:"warning,omitempty" jsonschema:"Non-fatal problem, e.g. no window/screenshot could be captured — the step was still recorded with just the note."`
}

func captureStepTool(_ context.Context, _ *mcp.CallToolRequest, in captureStepIn) (*mcp.CallToolResult, stepOut, error) {
	step, warning, err := engine.CaptureStep(in.Note)
	if err != nil {
		return nil, stepOut{}, err
	}
	return nil, stepOut{StepIndex: step.Index, WindowApp: step.WindowApp, WindowTitle: step.WindowTitle, Masked: step.Masked, Warning: warning}, nil
}

type runCommandIn struct {
	Command []string `json:"command" jsonschema:"Argv: Command[0] is the program, the rest are its arguments. No shell interpretation."`
}

type runCommandOut struct {
	StepIndex     int      `json:"step_index"`
	ExitCode      int      `json:"exit_code"`
	OutputPreview string   `json:"output_preview" jsonschema:"Possibly truncated head/tail preview of the command's output."`
	Masked        []string `json:"masked,omitempty"`
}

func runCommandTool(_ context.Context, _ *mcp.CallToolRequest, in runCommandIn) (*mcp.CallToolResult, runCommandOut, error) {
	if len(in.Command) == 0 {
		return nil, runCommandOut{}, fmt.Errorf("command must have at least one element")
	}
	step, exitCode, err := engine.RunCommand(in.Command, nil, nil) // no tee: stdout is the MCP protocol stream
	if err != nil {
		return nil, runCommandOut{}, err
	}
	return nil, runCommandOut{StepIndex: step.Index, ExitCode: exitCode, OutputPreview: step.Output, Masked: step.Masked}, nil
}

type stopRecordingOut struct {
	Name  string `json:"name"`
	Steps int    `json:"steps"`
}

func stopRecordingTool(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, stopRecordingOut, error) {
	s, err := engine.StopRecording()
	if err != nil {
		return nil, stopRecordingOut{}, err
	}
	return nil, stopRecordingOut{Name: s.Name, Steps: len(s.Steps)}, nil
}

type generateStoryIn struct {
	Name   string `json:"name" jsonschema:"The session name to render (as passed to start_recording)."`
	Format string `json:"format,omitempty" jsonschema:"'markdown' (default) or 'pdf'."`
}

type generateStoryOut struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty" jsonschema:"The rendered Markdown — include this directly in your response. Empty for PDF."`
}

func generateStoryTool(_ context.Context, _ *mcp.CallToolRequest, in generateStoryIn) (*mcp.CallToolResult, generateStoryOut, error) {
	if in.Name == "" {
		return nil, generateStoryOut{}, fmt.Errorf("name is required")
	}
	switch in.Format {
	case "", "markdown", "md":
		path, content, err := engine.ExportMarkdown(in.Name)
		if err != nil {
			return nil, generateStoryOut{}, err
		}
		return nil, generateStoryOut{Path: path, Content: content}, nil
	case "pdf":
		path, err := engine.ExportPDF(in.Name)
		if err != nil {
			return nil, generateStoryOut{}, err
		}
		return nil, generateStoryOut{Path: path}, nil
	default:
		return nil, generateStoryOut{}, fmt.Errorf("unknown format %q (want markdown or pdf)", in.Format)
	}
}

type windowInfoOut struct {
	App    string  `json:"app"`
	Title  string  `json:"title"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type listWindowsOut struct {
	Windows []windowInfoOut `json:"windows"`
}

func listWindowsTool(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listWindowsOut, error) {
	windows, err := capture.List()
	if err != nil {
		return nil, listWindowsOut{}, err
	}
	out := make([]windowInfoOut, len(windows))
	for i, w := range windows {
		out[i] = windowInfoOut{App: w.App, Title: w.Title, X: w.X, Y: w.Y, Width: w.W, Height: w.H}
	}
	return nil, listWindowsOut{Windows: out}, nil
}
