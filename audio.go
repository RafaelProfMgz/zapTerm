package main

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/normen/whatscli/config"
)

// audio playback state; guarded by audioMu. playingMsgId is read by the
// message renderer to mark the message that is currently playing.
var (
	audioMu      sync.Mutex
	audioCmd     *exec.Cmd
	playingMsgId string
)

// audioPlayerCandidates are tried in order when audio_command is not configured.
var audioPlayerCandidates = [][]string{
	{"mpv", "--no-video", "--really-quiet"},
	{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	{"play", "-q"},
	{"cvlc", "--play-and-exit", "--quiet"},
}

// audioPlayerCommand resolves the configured or auto-detected player command.
func audioPlayerCommand() ([]string, error) {
	if c := strings.TrimSpace(config.Config.General.AudioCommand); c != "" {
		return strings.Fields(c), nil
	}
	for _, candidate := range audioPlayerCandidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("nenhum player de áudio encontrado — instale mpv (recomendado), ffplay, sox ou vlc, ou configure audio_command em %s", config.GetConfigFilePath())
}

// playAudioFile plays path. Playing the message that is already playing stops
// it (toggle); playing a different message replaces the current playback.
func playAudioFile(path, msgId string) {
	audioMu.Lock()
	wasPlayingSame := audioCmd != nil && playingMsgId == msgId
	if audioCmd != nil {
		audioCmd.Process.Kill()
		audioCmd = nil
		playingMsgId = ""
	}
	if wasPlayingSame {
		audioMu.Unlock()
		queueRefreshChat()
		return
	}
	player, err := audioPlayerCommand()
	if err != nil {
		audioMu.Unlock()
		queuePrintError(err)
		return
	}
	cmd := exec.Command(player[0], append(player[1:], path)...)
	if err := cmd.Start(); err != nil {
		audioMu.Unlock()
		queuePrintError(fmt.Errorf("falha ao iniciar o player (%s): %v", player[0], err))
		return
	}
	audioCmd = cmd
	playingMsgId = msgId
	audioMu.Unlock()
	queueRefreshChat()

	go func() {
		cmd.Wait()
		audioMu.Lock()
		if audioCmd != cmd { // already stopped or replaced by the user
			audioMu.Unlock()
			return
		}
		audioCmd = nil
		playingMsgId = ""
		audioMu.Unlock()
		queueRefreshChat()
	}()
}

// stopAudio stops any playing audio.
func stopAudio() {
	audioMu.Lock()
	defer audioMu.Unlock()
	if audioCmd == nil {
		return
	}
	audioCmd.Process.Kill()
	audioCmd = nil
	playingMsgId = ""
}

// currentPlayingMsgId returns the id of the message being played, or "".
func currentPlayingMsgId() string {
	audioMu.Lock()
	defer audioMu.Unlock()
	return playingMsgId
}
