package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/livekit/protocol/auth"
)

const egressRequestTimeout = 15 * time.Second

// egressOutputDir is where the shared echo-recordings volume is mounted
// inside the egress container (deploy/compose/app.yml). Egress resolves a
// relative filepath against its own working directory, so anything but an
// absolute path here writes the MP4 outside the volume and the API never
// sees the file.
const egressOutputDir = "/output"

// EgressInfo is the subset of livekit.EgressInfo the API surfaces to clients.
type EgressInfo struct {
	EgressID string `json:"egressId,omitempty"`
	Status   string `json:"status,omitempty"`
}

// StartRecording launches a RoomComposite egress for the given LiveKit room
// and returns the egress id, which StopRecording needs.
//
// The recording path contract has three ends and they must agree:
//
//   - Egress writes the MP4 flat into egressOutputDir, an absolute path,
//     which is the echo-recordings volume inside that container.
//   - The egress_ended webhook stores on the call whatever filename LiveKit
//     reports, so an absolute /output/<name>.mp4.
//   - The API mounts the same volume read-only at RECORDINGS_DIR and opens
//     the stored path's basename inside that root. Flat writes make the
//     basename the volume-relative name, so neither end has to know the
//     other's mount point.
func (s *Service) StartRecording(ctx context.Context, roomName string) (EgressInfo, error) {
	token, err := s.recorderToken(roomName)
	if err != nil {
		return EgressInfo{}, err
	}
	file, _ := json.Marshal(map[string]any{
		"filepath": fmt.Sprintf("%s/%s-%d.mp4", egressOutputDir, sanitizeRoom(roomName), time.Now().Unix()),
	})
	body := map[string]any{
		"room_name":    roomName,
		"audio_only":   false,
		"file_outputs": []json.RawMessage{file},
	}
	var info EgressInfo
	if err := s.egressCall(ctx, token, "StartEgress", body, &info); err != nil {
		return EgressInfo{}, err
	}
	return info, nil
}

// StopRecording stops a running egress by id.
func (s *Service) StopRecording(ctx context.Context, egressID string) (EgressInfo, error) {
	token, err := s.recorderToken("")
	if err != nil {
		return EgressInfo{}, err
	}
	var info EgressInfo
	if err := s.egressCall(ctx, token, "StopEgress", map[string]any{"egress_id": egressID}, &info); err != nil {
		return EgressInfo{}, err
	}
	return info, nil
}

// recorderToken mints the server-side credential used to drive egress: full
// admin over one room when a room is given, unrestricted for StopEgress.
// Server-only credential: it is never exposed to clients and must not back
// any user-facing path.
func (s *Service) recorderToken(roomName string) (string, error) {
	video := &auth.VideoGrant{RoomJoin: false}
	if roomName != "" {
		video.Room = roomName
	}
	video.RoomAdmin = true
	video.RoomRecord = true
	token := auth.NewAccessToken(s.apiKey, s.apiSecret).
		SetIdentity("echo-recorder").
		SetValidFor(5 * time.Minute).
		SetVideoGrant(video)
	return token.ToJWT()
}

// httpURL converts a LiveKit ws:// or wss:// URL to its HTTP form, since the
// twirp endpoints are plain HTTP(S) on the same host.
func (s *Service) httpURL() string {
	u := s.url
	switch {
	case strings.HasPrefix(u, "wss://"):
		return "https://" + strings.TrimPrefix(u, "wss://")
	case strings.HasPrefix(u, "ws://"):
		return "http://" + strings.TrimPrefix(u, "ws://")
	default:
		return u
	}
}

func (s *Service) egressCall(ctx context.Context, token, method string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := s.httpURL()
	if url == "" {
		return errors.New("LIVEKIT_URL must be set")
	}
	url = strings.TrimSuffix(url, "/") + "/twirp/livekit.Egress/" + method
	reqCtx, cancel := context.WithTimeout(ctx, egressRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("livekit egress %s failed (status %d)", method, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func sanitizeRoom(roomName string) string {
	replaced := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, roomName)
	return replaced
}
