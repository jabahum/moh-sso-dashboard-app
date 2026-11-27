// Independent consumer: no host packages, source paths, migrations or SQLC types.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	a "github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements"
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
func ok(err error) {
	if err != nil {
		panic(err)
	}
}
func check(v bool, msg string) {
	if !v {
		panic(msg)
	}
}
func fail(err error, msg string)   { check(err != nil, msg) }
func text(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
func has(items []a.Announcement, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool := must(sql.Open("postgres", os.Getenv("ANNOUNCEMENTS_TEST_DSN")))
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	ok(pool.PingContext(ctx))
	_, err := a.NewRepository(nil)
	fail(err, "nil pool accepted")
	r := must(a.NewRepository(pool))
	actor := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	group := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	health := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	bad := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	client := uuid.New()
	by := uuid.NullUUID{UUID: actor, Valid: true}
	create := func(title string, audience a.AnnouncementAudienceType, status a.AnnouncementStatus) a.Announcement {
		return must(r.Create(ctx, a.CreateAnnouncementParams{
			Title: title, Message: "Fixture body", Level: a.AnnouncementLevelINFO,
			Status: status, AudienceType: audience, CreatedBy: actor,
		}))
	}
	item := create("Persistence fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusDRAFT)
	check(item.ID != uuid.Nil && item.Version == 1 && !item.Summary.Valid && !item.ExpiresAt.Valid, "create defaults/nulls")
	got := must(r.GetByID(ctx, item.ID))
	check(got == item, "get changed values")
	item = must(r.Update(ctx, a.UpdateAnnouncementParams{
		ID: item.ID, Title: "Updated fixture", Message: "Updated body", Summary: text("summary"),
		Level: a.AnnouncementLevelCRITICAL, Tag: text("tag"), LinkUrl: text("https://example.org"),
		LinkLabel: text("Open"), Priority: 4, IsPinned: true, AudienceType: a.AnnouncementAudienceTypeALLUSERS,
		NotifyByEmail: true, NotifyBySms: true, SmsMessage: text("SMS"), UpdatedBy: by,
		PublishAt: sql.NullTime{Time: time.Date(2020, 1, 2, 12, 0, 0, 0, time.FixedZone("EAT", 3*3600)), Valid: true},
	}))
	check(item.Summary.String == "summary" && item.Level == a.AnnouncementLevelCRITICAL &&
		item.LinkLabel.String == "Open" && item.NotifyByEmail && item.NotifyBySms &&
		item.SmsMessage.String == "SMS" && item.UpdatedBy == by && item.Version == 2, "update lost values")
	check(item.PublishAt.Valid && item.PublishAt.Time.Equal(time.Date(2020, 1, 2, 9, 0, 0, 0, time.UTC)), "timestamp offset lost")
	item = must(r.SetPinned(ctx, item.ID, false, actor))
	check(!item.IsPinned, "pin")
	item = must(r.SetPriority(ctx, item.ID, 8, actor))
	check(item.Priority == 8, "priority")
	item = must(r.Schedule(ctx, a.ScheduleAnnouncementParams{ID: item.ID,
		PublishAt: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, UpdatedBy: by}))
	check(item.Status == a.AnnouncementStatusSCHEDULED && item.PublishAt.Valid, "schedule")
	item = must(r.Draft(ctx, a.DraftAnnouncementParams{ID: item.ID, UpdatedBy: by}))
	check(item.Status == a.AnnouncementStatusDRAFT, "draft")
	item = must(r.PublishNow(ctx, item.ID, actor))
	check(item.Status == a.AnnouncementStatusPUBLISHED && item.PublishedAt.Valid && item.PublishedBy == by, "publish")
	// The draft still carried the future schedule time; publishing now must
	// not leave it hidden until then.
	check(item.PublishAt.Valid && !item.PublishAt.Time.After(time.Now().Add(time.Minute)), "publish now kept a future publish_at")

	// Due scheduled announcements are claimed exactly once; future and expired ones are not.
	due := create("Due fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusDRAFT)
	due = must(r.Schedule(ctx, a.ScheduleAnnouncementParams{ID: due.ID,
		PublishAt: sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true}, UpdatedBy: by}))
	future := create("Future fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusDRAFT)
	must(r.Schedule(ctx, a.ScheduleAnnouncementParams{ID: future.ID,
		PublishAt: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, UpdatedBy: by}))
	claimed := must(r.ClaimDueScheduled(ctx, 10))
	check(len(claimed) == 1 && claimed[0].ID == due.ID && claimed[0].Status == a.AnnouncementStatusPUBLISHED &&
		claimed[0].PublishedAt.Valid && claimed[0].PublishedBy == by, "claim due scheduled")
	check(len(must(r.ClaimDueScheduled(ctx, 10))) == 0, "due announcement claimed twice")
	// Remove the scheduling fixtures so the list and count checks below are unaffected.
	ok(r.SoftDelete(ctx, due.ID, actor))
	ok(r.SoftDelete(ctx, future.ID, actor))
	item = must(r.Archive(ctx, item.ID, actor))
	check(item.Status == a.AnnouncementStatusARCHIVED && item.ArchivedAt.Valid && item.ArchivedBy == by, "archive")
	item = must(r.UnarchiveToDraft(ctx, item.ID, actor))
	check(item.Status == a.AnnouncementStatusDRAFT, "unarchive")
	item = must(r.UpdateStatus(ctx, a.UpdateAnnouncementStatusParams{ID: item.ID, Status: a.AnnouncementStatusPUBLISHED, UpdatedBy: by}))
	check(item.Status == a.AnnouncementStatusPUBLISHED, "status")
	check(must(r.MarkEmailNotificationSent(ctx, item.ID)).EmailNotificationSentAt.Valid, "email marker")
	check(must(r.MarkSMSNotificationQueued(ctx, item.ID)).SmsNotificationQueuedAt.Valid, "SMS marker")
	sms := must(r.ListSMSRecipientsForUsers(ctx, []uuid.UUID{actor, actor, uuid.Nil,
		uuid.MustParse("10000000-0000-0000-0000-000000000002"),
		uuid.MustParse("10000000-0000-0000-0000-000000000003")}))
	check(len(sms) == 1 && sms[0].ID == actor && sms[0].PhoneNumber == "+256700000001", "SMS consent/phone filtering")
	check(len(must(r.ListSMSRecipientsForUsers(ctx, nil))) == 0, "empty SMS")
	check(len(must(r.ListSMSRecipientsForUsers(ctx, []uuid.UUID{uuid.Nil}))) == 0, "nil SMS IDs")

	// Audience add/replace deduplication, and rollback after partial writes.
	ok(r.AddClientAudience(ctx, item.ID, client))
	check(len(must(r.ListClientAudience(ctx, item.ID))) == 1, "add client")
	ok(r.ReplaceClientAudience(ctx, item.ID, []uuid.UUID{actor, actor}))
	ids := must(r.ListClientAudience(ctx, item.ID))
	check(len(ids) == 1 && ids[0] == actor, "client replace/dedup")
	fail(r.ReplaceClientAudience(ctx, item.ID, []uuid.UUID{client, bad}), "client replacement should fail")
	ids = must(r.ListClientAudience(ctx, item.ID))
	check(len(ids) == 1 && ids[0] == actor, "client rollback")
	ok(r.AddRoleAudience(ctx, item.ID, "admin"))
	check(len(must(r.ListRoleAudience(ctx, item.ID))) == 1, "add role")
	ok(r.ReplaceRoleAudience(ctx, item.ID, []string{"manager", "manager"}))
	roles := must(r.ListRoleAudience(ctx, item.ID))
	check(len(roles) == 1 && roles[0] == "manager", "role replace/dedup")
	fail(r.ReplaceRoleAudience(ctx, item.ID, []string{"user", strings.Repeat("x", 151)}), "role replacement should fail")
	roles = must(r.ListRoleAudience(ctx, item.ID))
	check(len(roles) == 1 && roles[0] == "manager", "role rollback")
	ok(r.AddUserAudience(ctx, item.ID, actor))
	check(len(must(r.ListUserAudience(ctx, item.ID))) == 1, "add user")
	ok(r.ReplaceUserAudience(ctx, item.ID, []uuid.UUID{client, client}))
	ids = must(r.ListUserAudience(ctx, item.ID))
	check(len(ids) == 1 && ids[0] == client, "user replace/dedup")
	fail(r.ReplaceUserAudience(ctx, item.ID, []uuid.UUID{actor, bad}), "user replacement should fail")
	ids = must(r.ListUserAudience(ctx, item.ID))
	check(len(ids) == 1 && ids[0] == client, "user rollback")
	ok(r.AddGroupAudience(ctx, item.ID, group))
	check(len(must(r.ListGroupAudience(ctx, item.ID))) == 1, "add group")
	ok(r.ReplaceGroupAudience(ctx, item.ID, []uuid.UUID{group, group, uuid.Nil}))
	check(len(must(r.ListGroupAudience(ctx, item.ID))) == 1, "group replace/dedup")
	fail(r.ReplaceGroupAudience(ctx, item.ID, []uuid.UUID{group, bad}), "group replacement should fail")
	ids = must(r.ListGroupAudience(ctx, item.ID))
	check(len(ids) == 1 && ids[0] == group, "group rollback")
	ok(r.ReplaceHealthContextAudience(ctx, item.ID, []uuid.UUID{health, health}, true))
	contexts := must(r.ListHealthContextAudience(ctx, item.ID))
	check(len(contexts) == 1 && contexts[0].ContextNodeID == health && contexts[0].IncludeDescendants, "health replace/dedup")
	fail(r.ReplaceHealthContextAudience(ctx, item.ID, []uuid.UUID{health, bad}, false), "health replacement should fail")
	contexts = must(r.ListHealthContextAudience(ctx, item.ID))
	check(len(contexts) == 1 && contexts[0].ContextNodeID == health && contexts[0].IncludeDescendants, "health rollback")
	ok(r.SaveAudienceSnapshot(ctx, item.ID, " email ", []string{actor.String()}))
	ok(r.SaveAudienceSnapshot(ctx, item.ID, "sms", []string{}))
	var count int
	var payload string
	ok(pool.QueryRowContext(ctx, "SELECT recipient_count,recipient_user_ids::text FROM announcement_audience_snapshots WHERE channel='EMAIL' AND announcement_id=$1", item.ID).Scan(&count, &payload))
	check(count == 1 && strings.Contains(payload, actor.String()), "snapshot JSON/channel")
	ok(pool.QueryRowContext(ctx, "SELECT recipient_count,recipient_user_ids::text FROM announcement_audience_snapshots WHERE channel='SMS' AND announcement_id=$1", item.ID).Scan(&count, &payload))
	check(count == 0 && payload == "[]", "empty snapshot")
	ok(r.ReplaceClientAudience(ctx, item.ID, nil))
	check(len(must(r.ListClientAudience(ctx, item.ID))) == 0, "clear client")
	ok(r.ReplaceRoleAudience(ctx, item.ID, nil))
	check(len(must(r.ListRoleAudience(ctx, item.ID))) == 0, "clear role")
	ok(r.ReplaceUserAudience(ctx, item.ID, nil))
	check(len(must(r.ListUserAudience(ctx, item.ID))) == 0, "clear user")
	ok(r.ReplaceGroupAudience(ctx, item.ID, nil))
	check(len(must(r.ListGroupAudience(ctx, item.ID))) == 0, "clear group")
	ok(r.ReplaceHealthContextAudience(ctx, item.ID, nil, false))
	check(len(must(r.ListHealthContextAudience(ctx, item.ID))) == 0, "clear health")

	attachment := must(r.CreateAttachment(ctx, a.CreateAnnouncementAttachmentParams{
		ID: uuid.New(), AnnouncementID: item.ID, FileName: "fixture.pdf", OriginalFileName: "Original.pdf",
		ContentType: text("application/pdf"), FileSize: 42, StorageProvider: "fixture", StorageKey: "fixture-key",
		Checksum: text("sha256-fixture"), UploadedBy: by, IncludeInEmail: true, Inline: true, ContentID: text("cid"), SortOrder: 2,
	}))
	check(attachment.Checksum.String == "sha256-fixture" && attachment.UploadedBy == by, "attachment values")
	check(must(r.CountAttachments(ctx, item.ID)) == 1, "attachment count")
	check(len(must(r.ListAttachmentsByAnnouncementID(ctx, item.ID))) == 1, "attachment list")
	check(len(must(r.ListEmailAttachments(ctx, item.ID))) == 1, "email attachment list")
	check(must(r.GetAttachmentByID(ctx, a.GetAnnouncementAttachmentByIDParams{ID: attachment.ID, AnnouncementID: item.ID})).ID == attachment.ID, "get attachment")
	_, err = r.GetAttachmentByID(ctx, a.GetAnnouncementAttachmentByIDParams{ID: attachment.ID, AnnouncementID: uuid.New()})
	check(errors.Is(err, sql.ErrNoRows), "attachment scope")
	attachment = must(r.UpdateAttachment(ctx, a.UpdateAnnouncementAttachmentParams{ID: attachment.ID,
		AnnouncementID: item.ID, IncludeInEmail: false, Inline: false, SortOrder: 9}))
	check(!attachment.IncludeInEmail && !attachment.Inline && !attachment.ContentID.Valid && attachment.SortOrder == 9, "update attachment/null")
	check(len(must(r.ListEmailAttachments(ctx, item.ID))) == 0, "email attachment exclusion")
	attachment = must(r.SoftDeleteAttachment(ctx, a.SoftDeleteAnnouncementAttachmentParams{
		ID: attachment.ID, AnnouncementID: item.ID, DeletedBy: by}))
	check(attachment.DeletedAt.Valid && attachment.DeletedBy == by, "delete attachment")
	check(must(r.CountAttachments(ctx, item.ID)) == 0 && len(must(r.ListAttachmentsByAnnouncementID(ctx, item.ID))) == 0, "deleted attachments visible")

	// Dedicated records prove audience isolation and active visibility rules.
	public := create("Public fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusPUBLISHED)
	targetedClient := create("Client fixture", a.AnnouncementAudienceTypeSPECIFICCLIENTS, a.AnnouncementStatusPUBLISHED)
	ok(r.AddClientAudience(ctx, targetedClient.ID, client))
	targetedRole := create("Role fixture", a.AnnouncementAudienceTypeSPECIFICROLES, a.AnnouncementStatusPUBLISHED)
	ok(r.AddRoleAudience(ctx, targetedRole.ID, "Manager"))
	targetedUser := create("User fixture", a.AnnouncementAudienceTypeSPECIFICUSERS, a.AnnouncementStatusPUBLISHED)
	ok(r.AddUserAudience(ctx, targetedUser.ID, actor))
	admin := create("Admin fixture", a.AnnouncementAudienceTypeADMINSONLY, a.AnnouncementStatusPUBLISHED)
	// Future and expired rows must not leak through any published feed.
	_, err = pool.ExecContext(ctx, "UPDATE announcements SET publish_at=now()+interval '1 day' WHERE id=$1", item.ID)
	ok(err)
	expired := create("Expired fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusPUBLISHED)
	_, err = pool.ExecContext(ctx, "UPDATE announcements SET expires_at=now()-interval '1 day' WHERE id=$1", expired.ID)
	ok(err)
	draft := create("Draft fixture", a.AnnouncementAudienceTypeALLUSERS, a.AnnouncementStatusDRAFT)
	check(must(r.CountAdmin(ctx)) == 8, "admin count")
	check(len(must(r.ListAdmin(ctx, a.ListAnnouncementsAdminParams{Limit: 20}))) == 8, "admin list")
	check(len(must(r.ListAdmin(ctx, a.ListAnnouncementsAdminParams{Limit: 1, Offset: 1}))) == 1, "admin pagination")
	check(must(r.CountByStatus(ctx, a.AnnouncementStatusDRAFT)) == 1, "status count")
	check(len(must(r.ListByStatus(ctx, a.ListAnnouncementsByStatusParams{Status: a.AnnouncementStatusDRAFT, Limit: 20}))) == 1, "status list")
	check(must(r.CountSearchAdmin(ctx, "Public fixture")) == 1, "search count")
	check(len(must(r.SearchAdmin(ctx, a.SearchAnnouncementsAdminParams{SearchText: text("public fixture"), PageLimit: 20}))) == 1, "case-insensitive search")
	check(must(r.CountCreatedByUser(ctx, actor)) == 8, "creator count")
	check(len(must(r.ListCreatedByUser(ctx, a.ListAnnouncementsCreatedByUserParams{CreatedBy: actor, Limit: 20}))) == 8, "creator list")
	check(must(r.CountCreatedByUser(ctx, bad)) == 0, "other creator count")
	active := must(r.ListActivePublished(ctx, a.ListActivePublishedAnnouncementsParams{Limit: 20}))
	check(len(active) == 5 && must(r.CountActivePublished(ctx)) == 5 && !has(active, item.ID) && !has(active, expired.ID) && !has(active, draft.ID), "active visibility/count")
	items := must(r.ListPublicAnnouncements(ctx, 20, 0))
	check(len(items) == 1 && has(items, public.ID), "public audience isolation")
	items = must(r.ListForClient(ctx, a.ListAnnouncementsForClientParams{ClientID: client, Limit: 20}))
	check(len(items) == 2 && has(items, targetedClient.ID) && has(items, public.ID), "client audience")
	check(len(must(r.ListForClient(ctx, a.ListAnnouncementsForClientParams{ClientID: bad, Limit: 20}))) == 1, "other client isolation")
	items = must(r.ListForRole(ctx, a.ListAnnouncementsForRoleParams{RoleName: "manager", PageLimit: 20}))
	check(len(items) == 2 && has(items, targetedRole.ID) && has(items, public.ID), "role audience")
	items = must(r.ListForRole(ctx, a.ListAnnouncementsForRoleParams{RoleName: "ADMIN", PageLimit: 20}))
	check(len(items) == 2 && has(items, admin.ID), "admin audience")
	items = must(r.ListForUser(ctx, a.ListAnnouncementsForUserParams{UserID: actor, Limit: 20}))
	check(len(items) == 2 && has(items, targetedUser.ID) && has(items, public.ID), "user audience")
	check(len(must(r.ListForUser(ctx, a.ListAnnouncementsForUserParams{UserID: bad, Limit: 20}))) == 1, "other user isolation")
	stats := must(r.GetStats(ctx))
	check(stats.Total == 8 && stats.PublishedCount == 7 && stats.DraftCount == 1 && stats.ActiveCount == 5, "stats")
	ok(r.SoftDelete(ctx, public.ID, actor))
	_, err = r.GetByID(ctx, public.ID)
	check(errors.Is(err, sql.ErrNoRows), "soft-deleted get")
	check(must(r.CountAdmin(ctx)) == 7 && len(must(r.ListPublicAnnouncements(ctx, 20, 0))) == 0, "soft-deleted lists")
	public = must(r.Restore(ctx, public.ID, actor))
	check(!public.DeletedAt.Valid && !public.DeletedBy.Valid, "restore")
	check(must(r.CountAdmin(ctx)) == 8, "restored count")
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = r.CountAdmin(canceled)
	check(errors.Is(err, context.Canceled), "cancellation")
	_, err = r.GetByID(ctx, uuid.New())
	check(errors.Is(err, sql.ErrNoRows), "missing record")
	// A failed write must not poison the pool or alter a successfully stored row.
	_, err = r.SetPriority(ctx, public.ID, -1, actor)
	fail(err, "priority constraint lost")
	check(must(r.GetByID(ctx, public.ID)).Priority == 0, "failed write persisted")
	ok(pool.PingContext(ctx))
	check(pool.Stats().InUse == 0, "connections leaked")
	fmt.Println("PASS: standalone Announcements PostgreSQL consumer; 54 operations, audience rollback, attachments, targeting, nulls, cancellation and caller-owned pool")
}
