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

// EgressInfo is the subset of livekit.EgressInfo the API surfaces to clients.
type EgressInfo struct {
	EgressID string `json:"egressId,omitempty"`
	Status   string `json:"status,omitempty"`
}

// StartRecording launches a RoomComposite egress for the given LiveKit room,
// writing an audio+video MP4 into the egress container's /output volume. It
// returns the egress id, which StopRecording needs.
func (s *Service) StartRecording(ctx context.Context, roomName string) (EgressInfo, error) {
	token, err := s.recorderToken(roomName)
	if err != nil {
		return EgressInfo{}, err
	}
	file, _ := json.Marshal(map[string]any{
		"filepath": fmt.Sprintf("recordings/%s-%d.mp4", sanitizeRoom(roomName), time.Now().Unix()),
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

func (s *Service) egressCall(ctx context.Context, token, method string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := strings.TrimSuffix(s.url, "/") + "/twirp/livekit.Egress/" + method
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
		return errors.New("livekit egress " + method + ": " + strings.TrimSpace(string(data)))
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
