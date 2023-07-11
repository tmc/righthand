# RightHand - Voice Controlled Assistant for Mac

RightHand is a voice controlled assistant for macOS, built using Go. It makes use of several powerful libraries such as `robotgo` for simulating keyboard input, `whisper` for voice recognition, `macdriver` for creating macOS applications using Go, and `langchaingo` for Language Learning Model interpretation. This software uses OpenAI's GPT-4 model to interpret transcriptions and generate corresponding commands.

## Motivation

<img width="218" alt="cyborg-tmc" src="https://github.com/tmc/righthand/assets/3977/5ac06331-48fc-4f53-8f0c-e1bfef000af8">

Two weeks before initially publishing this I got into a pretty bad mountain biking accident. I built this for myself to better use my computer with a mix of one-handed typing and voice control.

## Features

1. **Voice Recognition**: Leveraging the `whisper` model, RightHand can accurately transcribe spoken words into text.
2. **Language Learning Model**: RightHand uses `langchaingo`, with OpenAI's GPT-4 model to interpret the transcribed text and generate commands.
3. **Active App Context**: RightHand considers the context of the currently active application to interpret and execute commands appropriately.

## Installation

Ensure that Go is installed on your machine. To install RightHand, clone the repository and run:

```shell
$ go install github.com/tmc/righthand@main
```

## Usage

```shell
$ righthand
```

You can toggle the listening state of RightHand by pressing the control key while holding down the command key. RightHand will start transcribing your speech, interpret it, and execute commands on the active application.

## Architecture

```mermaid
graph TB
  User[User] -->|Voice Input + Hotkeys| RightHand

  subgraph RightHand Application
    RightHand -->|Toggles Listening| Audio[audioutil]
    Audio -->|Collects Audio Data| Whisper[whisper.cpp]
    Whisper -->|Transcribes Audio| RightHand
    RightHand -->|Monitors Key Events| NSApp[macdriver/NSApp]
    RightHand -->|Handles Text| LLM[langchaingo]
    RightHand -->|Simulates Key Presses| Robotgo[robotgo]
  end

  LLM -->|Interprets Transcription + Context| GPT4[OpenAI/GPT-4]
  GPT4 -->|Returns Key Presses| LLM

  classDef library fill:#bbc;
  class Audio,Cocoa,Robotgo,Whisper,LLM,NSApp library;
```

## Contributing

Contributions to RightHand are most welcome! If you have a feature request, bug report, or have developed a feature that you wish to be incorporated, please feel free to open a pull request.

