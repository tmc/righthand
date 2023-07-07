package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/progrium/macdriver/cocoa"
	"github.com/progrium/macdriver/objc"
	"github.com/tmc/audioutil/wavutil"
	"github.com/tmc/audioutil/whisperaudio"
	"github.com/tmc/audioutil/whisperutil"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/whisper.cpp/bindings/go/pkg/whisper"
)

const (
	// NSEventModifierFlagCommand is the command key modifier flag.
	NSEventModifierFlagCommand = 1 << 20
	// VKControl is the virtual key code for the control key.
	VKControl = 0x3B
	// VKCommand is the virtual key code for the command key.
	VKCommand = 0x37
	// VKOption is the virtual key code for the option key.
	VKOption = 0x3A
)

// App is the main application.
type App struct {
	listeningToggle chan struct{}
	wa              *whisperaudio.WhisperAudio
	llm             llms.ChatLLM
	cfg             *RightHandConfig
}

// newApp creates a new app.
func newApp(cfg RightHandConfig) (*App, error) {
	fmt.Fprintln(os.Stderr, "righthand: initializing...")
	fmt.Fprintln(os.Stderr, "righthand: using whisper model:", cfg.WhisperModel)
	wa, err := whisperaudio.New(
		whisperutil.WithAutoFetch(),
		whisperutil.WithModelName(cfg.WhisperModel),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create whisperaudio: %w", err)
	}
	cllm, err := openai.NewChat(openai.WithModel(cfg.LLMModel))
	if err != nil {
		return nil, fmt.Errorf("could not create chat LLM: %w", err)
	}
	return &App{
		listeningToggle: make(chan struct{}, 1),
		wa:              wa,
		llm:             cllm,
		cfg:             &cfg,
	}, nil
}

// run runs the app.
func (app *App) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go app.runMainLoop(ctx)
	app.runNSApp(ctx)
	return nil
}

// runMainLoop runs the main loop.
func (app *App) runMainLoop(ctx context.Context) {
	var (
		listening        bool
		listeningTimeout <-chan time.Time
		audioBuffer      []float32
	)
	fmt.Println("righthand: ready")
	for {
		select {
		case <-app.listeningToggle:
			listening = !listening
			if listening {
				listeningTimeout = time.After(defaultTimeout)
				fmt.Println("listening...")
				audioBuffer = nil
				err := app.wa.Start()
				if err != nil {
					log.Printf("error starting whisperaudio: %v", err)
				}
			} else {
				fmt.Println("transcribing...")
				if err := app.wa.Stop(); err != nil {
					log.Printf("error stopping whisperaudio: %v", err)
				}
				if app.cfg.DumpWAVFile {
					go wavutil.SaveWAV("output.wav", audioBuffer[:], whisper.SampleRate)
				}
				t1 := time.Now()
				text, err := app.wa.Transcribe(audioBuffer)
				if err != nil {
					log.Printf("error transcribing: %v", err)
					continue
				}
				fmt.Printf("transcribed: %q in %v\n", text, time.Since(t1))
				if text != "" {
					go app.handleText(ctx, text)
				}
			}
		case <-listeningTimeout:
			if listening {
				app.listeningToggle <- struct{}{}
			}
		case <-ctx.Done():
			fmt.Println("done")
			return
		default:
			if !listening {
				continue
			}
			buf, err := app.wa.CollectAudioData(time.Second)
			if err != nil {
				log.Printf("error collecting audio data: %v", err)
				continue
			}
			audioBuffer = append(audioBuffer, buf...)

		}
	}
}

// runNSApp runs the NSApp.
func (app *App) runNSApp(ctx context.Context) {
	nsApp := cocoa.NSApp_WithDidLaunch(func(n objc.Object) {
		events := make(chan cocoa.NSEvent, 64)
		go app.handleEvents(events)
		cocoa.NSEvent_GlobalMonitorMatchingMask(cocoa.NSEventMaskAny, events)
	})
	nsApp.ActivateIgnoringOtherApps(true)
	nsApp.Run()
}

// handleEvents handles global events.
func (app *App) handleEvents(events chan cocoa.NSEvent) {
	for {
		e := <-events
		typ := e.Get("type").Int()
		if typ != cocoa.NSEventTypeFlagsChanged {
			continue
		}
		app.manageListeningState(e)
	}
}

// manageListeningState toggles listening state.
func (app *App) manageListeningState(e cocoa.NSEvent) {
	keyCode := e.Get("keyCode").Int()
	modifierFlags := e.Get("modifierFlags").Int()
	cmdDown := modifierFlags&NSEventModifierFlagCommand != 0
	keyUp := !(modifierFlags&0x1 != 0)
	if (keyCode == VKControl) && cmdDown && keyUp {
		app.listeningToggle <- struct{}{}
	}
}

var systemPrompt = `You are an AI assistant that interprets transcribed voice input
and translates it into commands or text inputs for various applications. 

Your current active program is %v. Adjust your interpretation based on this context.

When interpreting commands, please indicate modifier keys such as Command, Option, Shift, 
or Control using curly braces. For instance, use '{Command}+t' for opening a new tab.

When outputting a command with a modifier key, use Shift as a modifier instead of including an uppercase character.

Your output will be used as keyboard input for the active application.
Return the input exactly as provided if you aren't confident in your answer.`

// handleText handles text.
func (app *App) handleText(ctx context.Context, text string) {
	activeApp := fmt.Sprint(cocoa.NSWorkspace_SharedWorkspace().FrontmostApplication().LocalizedName())
	fmt.Println("active app:", activeApp)

	messages := []schema.ChatMessage{
		schema.SystemChatMessage{
			Text: fmt.Sprintf(systemPrompt, activeApp),
		},
	}

	// check for few-shot examples for the active app from the config:
	// TODO(tmc): this would be faster as a map
	nExamples := 0
	for _, prog := range app.cfg.Programs {
		if prog.Program != activeApp {
			continue
		}
		for _, example := range prog.Examples {
			messages = append(messages, schema.HumanChatMessage{Text: example.Input})
			messages = append(messages, schema.AIChatMessage{Text: example.Output})
		}
		nExamples = len(prog.Examples)
	}

	fmt.Fprintf(os.Stderr, "righthand: using %v few-shot examples for %v\n", nExamples, activeApp)

	// append the human message:
	messages = append(messages, schema.HumanChatMessage{Text: text})

	llmText, err := app.llm.Call(ctx, messages)
	if err != nil {
		log.Printf("error calling LLM: %v", err)
		return
	}
	fmt.Println("response:", llmText)
	simulateTyping(llmText)
}

// keyTapPattern is a package-level compiled regular expression
//
// This regex is used to parse commands involving key presses.
// The pattern:
// 1. "\{" matches the literal opening brace
// 2. "((?:[^\\}]+\\+)*[^\\}]+)" matches one or more modifiers, each followed by a '+', except for the last one
// 3. "\\}" matches the literal closing brace
// 4. "(?:\\+([A-Za-z]+))?" optionally matches a key press (any sequence of letters) preceded by a '+'
// 5. "(?:[ ;])?" optionally matches a trailing space or semicolon
var keyTapPattern = regexp.MustCompile(`\{((?:[^\}]+\+)*[^\}]+)\}(?:\+([A-Za-z1-9]+))?(?:[ ;])?`)

// Helper function to simulate key tapping with given modifiers and key
func keyTapWithModifiers(modifiers []any, key string) {
	robotgo.KeySleep = 100
	robotgo.KeyTap(key, modifiers...)
	robotgo.KeyTap("shift")            // undo modifiers
	time.Sleep(100 * time.Millisecond) // slight delay to allow for key press to register
}

func extractModifiersAndKeyFromMatch(text string, match []int) ([]any, string) {
	// Map of modifiers to their representation for robotgo
	modifierMap := map[string]string{
		"Command": "command",
		"Shift":   "shift",
		"Option":  "alt",
		"Control": "ctrl",
		"Tab":     "tab",
		"Enter":   "enter",
	}

	// Extract the modifier keys
	modifierKeys := strings.Split(text[match[2]:match[3]], "+")
	modifiers := make([]any, 0, len(modifierKeys))
	key := ""

	// see if we have a key (check index 4)
	if match[4] != -1 {
		key = text[match[4]:match[5]]
	} else {
		key = modifierMap[modifierKeys[len(modifierKeys)-1]]
		modifierKeys = modifierKeys[:len(modifierKeys)-1] // Remove the last element (the key)
	}

	for _, modifier := range modifierKeys {
		modifierKey, exists := modifierMap[modifier]
		if !exists {
			log.Printf("Unknown modifier: %s", modifier)
			continue
		}
		modifiers = append(modifiers, modifierKey)
	}

	//fmt.Fprintln(os.Stderr, "righthand: modifiers:", modifiers, "key:", key)
	return modifiers, key
}

func simulateTyping(text string) {
	matches := keyTapPattern.FindAllStringSubmatchIndex(text, -1)

	lastIndex := 0
	for _, match := range matches {
		// Type the text before the match as normal
		if lastIndex != match[0] {
			fmt.Fprintln(os.Stderr, "righthand: typing text:", text[lastIndex:match[0]])
			robotgo.TypeStr(text[lastIndex:match[0]])
		}
		lastIndex = match[1] + 1 // Update lastIndex, adding 1 to ignore the trailing space

		modifiers, key := extractModifiersAndKeyFromMatch(text, match)

		// Simulate key press
		keyTapWithModifiers(modifiers, key)
	}

	// Type the rest of the text after the last match
	if lastIndex < len(text) {
		fmt.Fprintln(os.Stderr, "righthand: typing remainder of text:", text[lastIndex:])
		time.Sleep(100 * time.Millisecond) // slight delay to allow for key press to registerV
		robotgo.TypeStr(text[lastIndex:])
	}
}
