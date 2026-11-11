package deletions

import (
	"context"
	"database/sql"
)

// filesKey carries the recording files an erasure removes once its
// transaction has committed.
type filesKey struct{}

const selectRecordings = `SELECT x.filename FROM recordings x JOIN calls c ON c.id = x.call_id
	JOIN rooms r ON r.id = c.room_id WHERE r.owner_id = $1
	UNION SELECT c.recording_path FROM calls c JOIN rooms r ON r.id = c.room_id
	WHERE r.owner_id = $1 AND c.recording_path <> ''`

// statements erase one account's data, each taking the user id as $1. The
// seats are anonymised last: the statements before them find the person's
// lines, transcripts and summaries through the seat's user-<id> identity.
func statements() []string {
	return []string{
		`DELETE FROM rooms WHERE owner_id = $1`,
		`UPDATE transcript_lines l SET speaker = 'erased-' || p.id FROM call_participants p
			WHERE p.call_id = l.call_id AND p.identity = l.speaker AND p.identity = 'user-' || $1::bigint`,
		`UPDATE transcripts t
			SET content = regexp_replace(t.content, '(^|\n)' || p.identity || ': ', '\1erased-' || p.id || ': ', 'g')
			FROM call_participants p WHERE p.call_id = t.call_id AND p.identity = 'user-' || $1::bigint`,
		`UPDATE summaries s SET content = regexp_replace(s.content, '\m' || p.identity || '\M', 'erased-' || p.id, 'g')
			FROM call_participants p WHERE p.call_id = s.call_id AND p.identity = 'user-' || $1::bigint`,
		`DELETE FROM capture_consents WHERE identity = 'user-' || $1::bigint`,
		`UPDATE call_participants SET identity = 'erased-' || id, name = '', user_id = NULL
			WHERE user_id = $1 OR identity = 'user-' || $1::bigint`,
	}
}

// EraseData is porte/kit's EraseData hook: it deletes the rooms the person
// owns with their calls, transcripts, transcript lines, summaries,
// recordings, capture answers and seats, and strips the person from calls in
// rooms other people own. Their seat there loses the account link, the name
// and the user-<id> identity, and the same identity is replaced in the
// transcript lines, the transcript text and the summary, so nothing kept
// names them. The kit deletes the users row after it, in the same
// transaction. The recording files of the deleted calls are handed to the
// caller through the context, to be removed after the commit.
func EraseData(ctx context.Context, tx *sql.Tx, userID int64) error {
	files, err := recordingFiles(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, statement := range statements() {
		if _, err := tx.ExecContext(ctx, statement, userID); err != nil {
			return err
		}
	}
	if list, ok := ctx.Value(filesKey{}).(*[]string); ok {
		*list = append(*list, files...)
	}
	return nil
}

func recordingFiles(ctx context.Context, tx *sql.Tx, userID int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, selectRecordings, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var file string
		if err := rows.Scan(&file); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}
