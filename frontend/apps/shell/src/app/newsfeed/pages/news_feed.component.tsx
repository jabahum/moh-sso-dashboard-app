import { useMemo, useRef } from "react";
import {
  Document,
  Attachment,
  Help,
  Information,
  Pin,
  Time,
  ChevronRight,
  CheckmarkOutline,
  ErrorOutline,
  Warning,
  Login,
  Book,
  Bullhorn,
  Headset,
} from "@carbon/react/icons";
import { Button, Link, SkeletonText, Tag, Tile } from "@carbon/react";
import { useSelector } from "react-redux";
import { useNavigate } from "react-router-dom";

import { buildLoginURL, selectAuthenticated } from "@moh-sso/auth";
import { API } from "@moh-sso/config";
import { ErrorState } from "@moh-sso/ui";
import {
  buildUserAnnouncementAttachmentDownloadUrl,
  useListMyAnnouncementsQuery,
  useListPublicAnnouncementsQuery,
} from "@moh-sso/announcements/api";
import type {
  Announcement,
  AnnouncementLevel,
  AnnouncementStatus,
} from "@moh-sso/announcements/types";
import "./news-feed.scss";

const QUICK_LINKS = [
  {
    label: "Support System Portal",
    description: "Raise and track support tickets",
    href: "/support",
    icon: Headset,
  },
  {
    label: "Knowledgebase & FAQs",
    description: "Answers to common questions",
    href: "/faq",
    icon: Book,
  },
  {
    label: "Guidelines & Clinical Docs",
    description: "Policies, guidelines and SOPs",
    href: "/documents",
    icon: Document,
  },
];

type CarbonTagType =
  | "red"
  | "magenta"
  | "purple"
  | "blue"
  | "cyan"
  | "teal"
  | "green"
  | "gray"
  | "cool-gray"
  | "warm-gray"
  | "high-contrast"
  | "outline";

type TagConfig = {
  label: string;
  type: CarbonTagType;
};

function mapLevelTag(level: AnnouncementLevel): TagConfig {
  switch (level) {
    case "CRITICAL":
      return { label: "Critical", type: "red" };
    case "WARNING":
      return { label: "Warning", type: "warm-gray" };
    case "SUCCESS":
      return { label: "Success", type: "green" };
    case "INFO":
    default:
      return { label: "Info", type: "blue" };
  }
}

function mapStatusTag(status: AnnouncementStatus): TagConfig | null {
  switch (status) {
    case "PUBLISHED":
      return null;
    case "SCHEDULED":
      return { label: "Scheduled", type: "purple" };
    case "DRAFT":
      return { label: "Draft", type: "cool-gray" };
    case "ARCHIVED":
      return { label: "Archived", type: "gray" };
    default:
      return null;
  }
}

function mapCustomTag(tag?: string | null): TagConfig | null {
  const label = tag?.trim();

  if (!label) {
    return null;
  }

  switch (label.toLowerCase()) {
    case "critical":
    case "urgent":
    case "alert":
    case "action":
      return { label, type: "red" };

    case "scheduled":
    case "maintenance":
    case "warning":
      return { label, type: "warm-gray" };

    case "event":
    case "new":
    case "update":
    case "upgrade":
      return { label, type: "blue" };

    case "success":
    case "resolved":
    case "completed":
      return { label, type: "green" };

    default:
      return { label, type: "gray" };
  }
}

function getAnnouncementDate(item: Announcement): string | null {
  return item.publish_at || item.created_at || null;
}

function getAnnouncementTime(item: Announcement): number {
  const rawDate = getAnnouncementDate(item);

  if (!rawDate) {
    return 0;
  }

  const time = new Date(rawDate).getTime();

  return Number.isNaN(time) ? 0 : time;
}

function formatTimestamp(dateString?: string | null): string {
  if (!dateString) {
    return "Recently posted";
  }

  const date = new Date(dateString);

  if (Number.isNaN(date.getTime())) {
    return "Recently posted";
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function formatRelativeTimestamp(dateString?: string | null): string {
  if (!dateString) {
    return "Recently posted";
  }

  const date = new Date(dateString);

  if (Number.isNaN(date.getTime())) {
    return "Recently posted";
  }

  const diffMs = Date.now() - date.getTime();

  if (diffMs < 0) {
    return formatTimestamp(dateString);
  }

  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSeconds < 60) {
    return "Just now";
  }

  if (diffMinutes < 60) {
    return `${diffMinutes} minute${diffMinutes === 1 ? "" : "s"} ago`;
  }

  if (diffHours < 24) {
    return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`;
  }

  if (diffDays < 7) {
    return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
  }

  return formatTimestamp(dateString);
}

function formatFileSize(bytes?: number): string {
  if (!Number.isFinite(bytes) || !bytes || bytes <= 0) {
    return "";
  }

  if (bytes < 1024) {
    return `${bytes} B`;
  }

  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }

  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function isExternalHref(href: string): boolean {
  return /^https?:\/\//i.test(href);
}

function isPlaceholderHref(href: string): boolean {
  return href.trim() === "#";
}

interface AppLinkProps {
  href: string;
  className?: string;
  children: React.ReactNode;
}

function AppLink({ href, className, children }: AppLinkProps) {
  const navigate = useNavigate();

  if (isPlaceholderHref(href)) {
    return (
      <button type="button" className={className} disabled>
        {children}
      </button>
    );
  }

  if (isExternalHref(href)) {
    return (
      <Link href={href} target="_blank" rel="noopener noreferrer" className={className}>
        {children}
      </Link>
    );
  }

  return (
    <button type="button" className={className} onClick={() => navigate(href)}>
      {children}
    </button>
  );
}

function AnnouncementLink({ item }: { item: Announcement }) {
  const href = item.link_url?.trim();
  const label = item.link_label?.trim() || "Read more";

  if (!href) {
    return null;
  }

  return (
    <div className="feed-link-row">
      <span className="feed-link-row__label">Related link</span>
      <AppLink href={href} className="feed-link">
        {label}
        <ChevronRight size={16} />
      </AppLink>
    </div>
  );
}

function getLevelIcon(level: AnnouncementLevel) {
  switch (level) {
    case "CRITICAL":
      return ErrorOutline;
    case "WARNING":
      return Warning;
    case "SUCCESS":
      return CheckmarkOutline;
    case "INFO":
    default:
      return Information;
  }
}

function AnnouncementCard({ item }: { item: Announcement }) {
  const LevelIcon = getLevelIcon(item.level);
  const levelTag = mapLevelTag(item.level);
  const customTag = mapCustomTag(item.tag);
  const statusTag = mapStatusTag(item.status);
  const timestampSource = getAnnouncementDate(item);
  const attachmentCount = item.attachment_count ?? item.attachments?.length ?? 0;
  const hasAttachmentList = Boolean(item.attachments && item.attachments.length > 0);

  return (
    <Tile
      className={[
        "feed-item",
        `feed-item--${item.level.toLowerCase()}`,
        item.is_pinned ? "feed-item--pinned" : "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <div className="feed-item__top">
        <div className="feed-item__title-row">
          <span
            className={[
              "feed-item__icon-box",
              `feed-item__icon-box--${item.level.toLowerCase()}`,
            ].join(" ")}
            aria-hidden="true"
          >
            <LevelIcon size={18} />
          </span>

          <div className="feed-item__title-content">
            <div className="feed-item__title-line">
              <h4 className="feed-item__title">{item.title}</h4>

              {item.is_pinned && (
                <Tag type="warm-gray" size="sm">
                  <span className="feed-tag-with-icon">
                    <Pin size={12} />
                    Pinned
                  </span>
                </Tag>
              )}

              <Tag type={levelTag.type} size="sm">
                {levelTag.label}
              </Tag>

              {customTag && (
                <Tag type={customTag.type} size="sm">
                  {customTag.label}
                </Tag>
              )}

              {statusTag && (
                <Tag type={statusTag.type} size="sm">
                  {statusTag.label}
                </Tag>
              )}

              {attachmentCount > 0 && (
                <Tag type="cyan" size="sm">
                  <span className="feed-tag-with-icon">
                    <Attachment size={12} />
                    {attachmentCount === 1 ? "1 file" : `${attachmentCount} files`}
                  </span>
                </Tag>
              )}
            </div>

            {item.summary && <p className="feed-summary">{item.summary}</p>}

            <p className="feed-message">{item.message}</p>

            <AnnouncementLink item={item} />

            {attachmentCount > 0 && (
              <div className="feed-attachments">
                <div className="feed-attachments__title">
                  <Attachment size={14} />
                  <span>
                    {attachmentCount === 1 ? "Attachment" : `Attachments (${attachmentCount})`}
                  </span>
                </div>

                {hasAttachmentList ? (
                  <div className="feed-attachments__list">
                    {item.attachments?.map((attachment) => {
                      const href =
                        attachment.download_url ||
                        buildUserAnnouncementAttachmentDownloadUrl(item.id, attachment.id);
                      const sizeLabel = formatFileSize(attachment.file_size);

                      return (
                        <Link
                          key={attachment.id}
                          href={href}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="feed-attachment-link"
                        >
                          <Document size={14} />
                          <span>{attachment.original_file_name || attachment.file_name}</span>
                          {sizeLabel && <small>{sizeLabel}</small>}
                        </Link>
                      );
                    })}
                  </div>
                ) : (
                  <p className="feed-attachments__empty">
                    This announcement has {attachmentCount} attachment
                    {attachmentCount === 1 ? "" : "s"}.
                  </p>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="feed-timestamp" title={formatTimestamp(timestampSource)}>
          <Time size={14} />
          <span>{formatRelativeTimestamp(timestampSource)}</span>
        </div>
      </div>
    </Tile>
  );
}

function LoadingFeed() {
  return (
    <div className="feed-section-list" aria-label="Loading announcements">
      {Array.from({ length: 3 }).map((_, index) => (
        <Tile className="feed-item feed-item--loading" key={index}>
          <SkeletonText width="45%" />
          <SkeletonText paragraph lineCount={3} />
          <SkeletonText width="25%" />
        </Tile>
      ))}
    </div>
  );
}

function EmptyFeed() {
  return (
    <div className="feed-empty" role="status">
      <span className="feed-empty__icon" aria-hidden="true">
        <Bullhorn size={24} />
      </span>
      <div>
        <h4 className="feed-empty__title">No announcements yet</h4>
        <p className="feed-empty__description">
          System updates and important notices will appear here when they are published.
        </p>
      </div>
    </div>
  );
}

function SignInIntro() {
  const loginStartedRef = useRef(false);

  const handleLogin = () => {
    if (loginStartedRef.current) {
      return;
    }

    loginStartedRef.current = true;
    window.location.assign(buildLoginURL(API.auth.login()));
  };

  return (
    <Tile className="news-intro">
      <div className="news-intro__text">
        <h4 className="news-intro__title">One sign-in for Ministry of Health systems</h4>
        <p className="news-intro__description">
          Sign in with your Ministry of Health account to open the health information systems you
          have access to and get help from support.
        </p>
      </div>

      <Button kind="primary" size="md" renderIcon={Login} onClick={handleLogin}>
        Sign in
      </Button>
    </Tile>
  );
}

interface FeedSectionProps {
  title: string;
  subtitle?: string;
  icon: React.ElementType;
  count?: number;
  children: React.ReactNode;
}

function FeedSection({ title, subtitle, icon: Icon, count, children }: FeedSectionProps) {
  return (
    <section className="feed-section">
      <div className="feed-section-header">
        <div>
          <div className="feed-section-title">
            <Icon size={16} />
            <h4>
              {title}
              {typeof count === "number" ? ` (${count})` : ""}
            </h4>
          </div>

          {subtitle && <p className="feed-section-subtitle">{subtitle}</p>}
        </div>
      </div>

      <div className="feed-section-list">{children}</div>
    </section>
  );
}

export default function NewsFeedPage() {
  const navigate = useNavigate();
  const isAuthenticated = useSelector(selectAuthenticated);

  const publicQuery = useListPublicAnnouncementsQuery(
    { limit: 20, offset: 0 },
    {
      refetchOnMountOrArgChange: true,
    },
  );

  const myQuery = useListMyAnnouncementsQuery(
    { limit: 20, offset: 0 },
    {
      refetchOnMountOrArgChange: true,
      skip: !isAuthenticated,
    },
  );

  const announcements = useMemo<Announcement[]>(() => {
    const byId = new Map<string, Announcement>();

    for (const item of publicQuery.data ?? []) {
      byId.set(item.id, item);
    }

    for (const item of myQuery.data ?? []) {
      byId.set(item.id, item);
    }

    return Array.from(byId.values());
  }, [myQuery.data, publicQuery.data]);

  const sortedAnnouncements = useMemo(() => {
    return [...announcements].sort((a, b) => {
      if (a.is_pinned !== b.is_pinned) {
        return a.is_pinned ? -1 : 1;
      }

      return getAnnouncementTime(b) - getAnnouncementTime(a);
    });
  }, [announcements]);

  const pinnedAnnouncements = useMemo(
    () => sortedAnnouncements.filter((item) => item.is_pinned),
    [sortedAnnouncements],
  );

  const regularAnnouncements = useMemo(
    () => sortedAnnouncements.filter((item) => !item.is_pinned),
    [sortedAnnouncements],
  );

  const isLoadingAnnouncements = publicQuery.isLoading || (isAuthenticated && myQuery.isLoading);
  const isFetchingAnnouncements = publicQuery.isFetching || (isAuthenticated && myQuery.isFetching);

  const shouldShowInitialLoading = isLoadingAnnouncements && announcements.length === 0;
  const shouldShowRefreshing = isFetchingAnnouncements && announcements.length > 0;

  const hasError = publicQuery.isError && myQuery.isError && announcements.length === 0;

  const isEmpty =
    !isLoadingAnnouncements &&
    !isFetchingAnnouncements &&
    !hasError &&
    pinnedAnnouncements.length === 0 &&
    regularAnnouncements.length === 0;

  const handleRetry = () => {
    publicQuery.refetch();
    if (isAuthenticated) {
      myQuery.refetch();
    }
  };

  return (
    <div className="page-container news-page">
      <header className="page-header news-page__header">
        <div>
          <h3 className="page-title">News & Updates</h3>
          <p className="page-subtitle">Latest system updates, announcements, and notices.</p>
        </div>

        {shouldShowRefreshing && (
          <Tag type="blue" size="sm">
            Refreshing
          </Tag>
        )}
      </header>

      <section className="page-content news-page__content">
        <main className="news-feed">
          {!isAuthenticated && <SignInIntro />}

          {shouldShowInitialLoading && <LoadingFeed />}

          {hasError && (
            <ErrorState
              title="Failed to load announcements"
              description="Unable to load announcements at the moment."
              primaryAction={{
                label: "Retry",
                onClick: handleRetry,
              }}
              secondaryAction={{
                label: "Contact support",
                onClick: () => navigate("/support"),
              }}
            />
          )}

          {isEmpty && <EmptyFeed />}

          {!hasError && !shouldShowInitialLoading && pinnedAnnouncements.length > 0 && (
            <FeedSection title="Pinned Announcements" icon={Pin} count={pinnedAnnouncements.length}>
              {pinnedAnnouncements.map((item) => (
                <AnnouncementCard key={item.id} item={item} />
              ))}
            </FeedSection>
          )}

          {!hasError && !shouldShowInitialLoading && regularAnnouncements.length > 0 && (
            <FeedSection
              title="All Announcements"
              icon={Information}
              count={regularAnnouncements.length}
            >
              {regularAnnouncements.map((item) => (
                <AnnouncementCard key={item.id} item={item} />
              ))}
            </FeedSection>
          )}
        </main>

        <aside className="news-sidebar">
          <Tile className="news-sidebar__tile">
            <div className="news-sidebar__heading">
              <Document size={18} />
              <h4>Quick Links</h4>
            </div>

            <ul className="sidebar-link-list">
              {QUICK_LINKS.map(({ label, description, href, icon: Icon }) => (
                <li key={label}>
                  <AppLink href={href} className="sidebar-link">
                    <span className="sidebar-link__label">
                      <span className="sidebar-link__icon" aria-hidden="true">
                        <Icon size={16} />
                      </span>
                      <span className="sidebar-link__text">
                        <span className="sidebar-link__title">{label}</span>
                        <span className="sidebar-link__description">{description}</span>
                      </span>
                    </span>
                    <ChevronRight size={16} />
                  </AppLink>
                </li>
              ))}
            </ul>
          </Tile>

          <Tile className="news-sidebar__tile news-sidebar__help">
            <div className="news-sidebar__heading">
              <Help size={18} />
              <h4>Need Help?</h4>
            </div>

            <p className="sidebar-help-text">
              Contact support for help with account access, passwords, or reporting a problem with
              the portal.
            </p>

            <Button
              kind="tertiary"
              size="md"
              className="news-sidebar__support-button"
              onClick={() => navigate("/support")}
            >
              Contact support
            </Button>
          </Tile>
        </aside>
      </section>
    </div>
  );
}
