// SPDX-License-Identifier: Apache-2.0
package mmfederation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *Service) importedFiles(ctx context.Context, p Peer, l Link, ids []string) ([]string, error) {
	if len(ids) > chat.MaxFilesPerPost {
		return nil, ErrInvalid
	}
	out := []string{}
	for _, id := range ids {
		if !model.IsValidId(id) {
			return nil, ErrInvalid
		}
		var local string
		e := s.db.QueryRowContext(ctx, s.q(`SELECT local_file_id FROM chat_federation_files WHERE peer_id=? AND link_id=? AND remote_file_id=?`), p.ID, l.ID, id).Scan(&local)
		if e != nil || local == "" {
			return nil, ErrInvalid
		}
		out = append(out, local)
	}
	return out, nil
}
func (s *Service) saveFile(ctx context.Context, p Peer, l Link, user *storage.User, id, name string, size int64, reader io.Reader) (chat.Attachment, error) {
	if !model.IsValidId(id) || size < 0 || size > chat.MaxAttachmentBytes || len(name) > 255 || strings.ContainsAny(name, "/\\\r\n\x00") {
		return chat.Attachment{}, ErrInvalid
	}
	if e := s.active(ctx, p, l); e != nil {
		return chat.Attachment{}, e
	}
	staged, e := s.Chat.StageAttachment(name, io.LimitReader(reader, size+1), size)
	if e != nil {
		return chat.Attachment{}, e
	}
	defer staged.Discard()
	if staged.Size() != size {
		return chat.Attachment{}, ErrInvalid
	}
	files, e := s.Chat.UploadAttachments(ctx, user, l.ChannelID, []chat.StagedUpload{staged})
	if e != nil {
		return chat.Attachment{}, e
	}
	file := files[0]
	_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_files(upload_id,peer_id,link_id,remote_file_id,user_id,filename,size,created_at,local_file_id) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(link_id,remote_file_id) DO UPDATE SET local_file_id=CASE WHEN chat_federation_files.local_file_id='' THEN excluded.local_file_id ELSE chat_federation_files.local_file_id END`), model.NewId(), p.ID, l.ID, id, user.ID, name, size, time.Now().UnixMilli(), file.ID)
	if e != nil {
		return file, e
	}
	var actual string
	if e = s.db.QueryRowContext(ctx, s.q(`SELECT local_file_id FROM chat_federation_files WHERE link_id=? AND remote_file_id=?`), l.ID, id).Scan(&actual); e != nil {
		return file, e
	}
	if actual != file.ID {
		file, _, e = s.Chat.Attachment(ctx, user, actual, false)
	}
	return file, e
}
func (s *Service) createNativeUpload(w http.ResponseWriter, r *http.Request, p Peer, raw []byte) {
	var upload model.UploadSession
	if json.Unmarshal(raw, &upload) != nil || upload.Type != model.UploadTypeAttachment || !model.IsValidId(upload.UserId) || !model.IsValidId(upload.ChannelId) || !model.IsValidId(upload.ReqFileId) || upload.FileSize < 0 || upload.FileSize > chat.MaxAttachmentBytes || len(upload.Filename) == 0 || len(upload.Filename) > 255 || strings.ContainsAny(upload.Filename, "/\\\r\n\x00") {
		nativeReply(w, 400, nil, ErrInvalid)
		return
	}
	l, e := scanLink(s.db.QueryRowContext(r.Context(), s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? AND remote_channel_id=? AND enabled=1`), p.ID, upload.ChannelId))
	if e != nil || l.ReadOnly || s.active(r.Context(), p, l) != nil {
		nativeReply(w, 403, nil, ErrForbidden)
		return
	}
	u, e := s.remoteUser(r.Context(), p, upload.UserId, nil, false)
	if e == nil {
		e = s.member(r.Context(), p, l, u)
	}
	if e != nil {
		nativeReply(w, 400, nil, e)
		return
	}
	var oldID, filename, local, user string
	var size int64
	e = s.db.QueryRowContext(r.Context(), s.q(`SELECT upload_id,filename,size,local_file_id,user_id FROM chat_federation_files WHERE link_id=? AND remote_file_id=?`), l.ID, upload.ReqFileId).Scan(&oldID, &filename, &size, &local, &user)
	if e == nil {
		if filename != upload.Filename || size != upload.FileSize || user != u.ID {
			nativeReply(w, 409, nil, ErrInvalid)
			return
		}
		// Reissue abandoned pending sessions so a retry after expiry can recover.
		if local == "" {
			_, e = s.db.ExecContext(r.Context(), s.q(`UPDATE chat_federation_files SET created_at=? WHERE upload_id=? AND local_file_id=''`), time.Now().UnixMilli(), oldID)
		}
		upload.Id = oldID
		if local != "" {
			upload.FileOffset = size
		}
	} else if errors.Is(e, sql.ErrNoRows) {
		upload.Id = model.NewId()
		_, e = s.db.ExecContext(r.Context(), s.q(`INSERT INTO chat_federation_files(upload_id,peer_id,link_id,remote_file_id,user_id,filename,size,created_at) VALUES(?,?,?,?,?,?,?,?)`), upload.Id, p.ID, l.ID, upload.ReqFileId, u.ID, upload.Filename, upload.FileSize, time.Now().UnixMilli())
	} else {
		nativeReply(w, 500, nil, e)
		return
	}
	if e != nil {
		nativeReply(w, 500, nil, e)
		return
	}
	upload.Path = ""
	upload.RemoteId = p.ID
	nativeReply(w, 200, upload, nil)
}
func (s *Service) receiveNativeFile(w http.ResponseWriter, r *http.Request, p Peer) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v4/remotecluster/upload/")
	if !model.IsValidId(id) {
		nativeReply(w, 400, nil, ErrInvalid)
		return
	}
	var linkID, foreign, userID, name, local string
	var size, created int64
	e := s.db.QueryRowContext(r.Context(), s.q(`SELECT link_id,remote_file_id,user_id,filename,size,created_at,local_file_id FROM chat_federation_files WHERE upload_id=? AND peer_id=?`), id, p.ID).Scan(&linkID, &foreign, &userID, &name, &size, &created, &local)
	l, e2 := s.Link(r.Context(), p, linkID)
	if e != nil || e2 != nil || s.active(r.Context(), p, l) != nil || (local == "" && time.Now().UnixMilli()-created > 24*60*60*1000) {
		nativeReply(w, 403, nil, ErrForbidden)
		return
	}
	u, e := s.Store.GetUserByID(r.Context(), userID)
	if e != nil {
		nativeReply(w, 403, nil, e)
		return
	}
	var file chat.Attachment
	if local != "" {
		if r.ContentLength != 0 {
			n, err := io.Copy(io.Discard, io.LimitReader(r.Body, size+1))
			if err != nil || n != size {
				nativeReply(w, 400, nil, ErrInvalid)
				return
			}
		}
		file, _, e = s.Chat.Attachment(r.Context(), u, local, false)
	} else {
		file, e = s.saveFile(r.Context(), p, l, u, foreign, name, size, http.MaxBytesReader(w, r.Body, chat.MaxAttachmentBytes+1))
	}
	if e != nil {
		nativeReply(w, 400, nil, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fileModel(file, foreign))
}
func fileModel(file chat.Attachment, id string) *model.FileInfo {
	return &model.FileInfo{Id: id, CreatorId: chat.ID(file.UserID), PostId: file.PostID, CreateAt: file.CreatedAt, UpdateAt: file.CreatedAt, Name: file.Name, Size: file.Size, MimeType: file.MIMEType, Width: file.Width, Height: file.Height, HasPreviewImage: file.HasPreview()}
}
func (s *Service) exportNativeFile(ctx context.Context, p Peer, l Link, post chat.Post, file chat.Attachment, user *model.User) error {
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil {
		return e
	}
	_, data, e := s.Chat.Attachment(ctx, owner, file.ID, true)
	if e != nil {
		return e
	}
	defer data.Close()
	session := model.UploadSession{Id: model.NewId(), Type: model.UploadTypeAttachment, UserId: user.Id, ChannelId: l.RemoteChannelID, Filename: file.Name, FileSize: file.Size, RemoteId: p.ID, ReqFileId: file.ID}
	var received model.UploadSession
	if e = s.sendFrame(ctx, p, "/api/v4/remotecluster/msg", "sharedchannel_upload", session, &received); e != nil {
		return e
	}
	if !model.IsValidId(received.Id) {
		return ErrInvalid
	}
	if received.FileOffset == file.Size {
		return nil
	}
	if received.FileOffset != 0 {
		return ErrInvalid
	}
	response, e := s.request(ctx, p, "POST", "/api/v4/remotecluster/upload/"+received.Id, data, "application/octet-stream")
	if e != nil {
		return e
	}
	defer response.Body.Close()
	var info model.FileInfo
	if e = decodeJSON(response.Body, &info, 1<<20); e != nil {
		return e
	}
	if info.Id != file.ID {
		return ErrInvalid
	}
	return nil
}
func (s *Service) pullRESTFiles(ctx context.Context, p Peer, l Link, post *model.Post, user *storage.User) ([]string, error) {
	if len(post.FileIds) > chat.MaxFilesPerPost {
		return nil, ErrInvalid
	}
	out := []string{}
	for _, id := range post.FileIds {
		if !model.IsValidId(id) {
			return nil, ErrInvalid
		}
		var local string
		e := s.db.QueryRowContext(ctx, s.q(`SELECT local_file_id FROM chat_federation_files WHERE link_id=? AND remote_file_id=?`), l.ID, id).Scan(&local)
		if e == nil && local != "" {
			out = append(out, local)
			continue
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		response, e := s.request(ctx, p, "GET", "/api/v4/files/"+id+"/info", nil, "")
		if e != nil {
			return nil, e
		}
		var info model.FileInfo
		e = decodeJSON(response.Body, &info, 1<<20)
		response.Body.Close()
		if e != nil || info.Id != id || !model.IsValidId(info.PostId) {
			return nil, ErrInvalid
		}
		if info.PostId != post.Id {
			response, e = s.request(ctx, p, "GET", "/api/v4/posts/"+info.PostId, nil, "")
			if e != nil {
				return nil, e
			}
			var referenced model.Post
			e = decodeJSON(response.Body, &referenced, 1<<20)
			response.Body.Close()
			if e != nil || referenced.ChannelId != l.RemoteChannelID {
				return nil, ErrForbidden
			}
		}
		response, e = s.request(ctx, p, "GET", "/api/v4/files/"+id, nil, "")
		if e != nil {
			return nil, e
		}
		saved, e := s.saveFile(ctx, p, l, user, id, info.Name, info.Size, response.Body)
		response.Body.Close()
		if e != nil {
			return nil, e
		}
		out = append(out, saved.ID)
	}
	return out, nil
}
func (s *Service) pushRESTFiles(ctx context.Context, p Peer, l Link, post chat.Post) ([]string, error) {
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, file := range post.Attachments {
		_, content, e := s.Chat.Attachment(ctx, owner, file.ID, true)
		if e != nil {
			return nil, e
		}
		reader, writer := io.Pipe()
		multipartWriter := multipart.NewWriter(writer)
		done := make(chan error, 1)
		go func() {
			defer content.Close()
			e := multipartWriter.WriteField("channel_id", l.RemoteChannelID)
			if e == nil {
				var part io.Writer
				part, e = multipartWriter.CreateFormFile("files", file.Name)
				if e == nil {
					_, e = io.Copy(part, content)
				}
			}
			if e == nil {
				e = multipartWriter.Close()
			}
			writer.CloseWithError(e)
			done <- e
		}()
		response, e := s.request(ctx, p, "POST", "/api/v4/files", reader, multipartWriter.FormDataContentType())
		reader.CloseWithError(e)
		copyErr := <-done
		if e != nil {
			return nil, e
		}
		if copyErr != nil {
			response.Body.Close()
			return nil, copyErr
		}
		var result struct {
			FileInfos []*model.FileInfo `json:"file_infos"`
		}
		e = decodeJSON(response.Body, &result, 1<<20)
		response.Body.Close()
		if e != nil || len(result.FileInfos) != 1 || !model.IsValidId(result.FileInfos[0].Id) {
			return nil, ErrInvalid
		}
		out = append(out, result.FileInfos[0].Id)
	}
	return out, nil
}
