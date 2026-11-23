type SurveillanceTileItem = {
  label: string;
  href?: string;
  onClick?: () => void;
};

type SurveillanceTile = {
  title: string;
  items: SurveillanceTileItem[];
  /** Total number of matching diseases; may exceed `items.length` when the list is truncated. */
  total?: number;
};

type SurveillanceTileTone = "immediate-action" | "take-action" | "alert" | "watch";

interface SurveillanceTilesProps {
  immediateAction?: SurveillanceTile;
  takeAction?: SurveillanceTile;
  alert?: SurveillanceTile;
  watch?: SurveillanceTile;
  loading?: boolean;
}

function TileList({
  title,
  items,
  total = items.length,
  tone,
  loading = false,
}: SurveillanceTile & { tone: SurveillanceTileTone; loading?: boolean }) {
  const isEmpty = !loading && total === 0;
  const hiddenCount = Math.max(total - items.length, 0);
  const headingId = `surveillance-tile-${tone}-title`;
  const countLabel = loading
    ? "Loading"
    : total === 0
      ? "No data"
      : `${total} ${total === 1 ? "disease" : "diseases"}`;

  return (
    <section
      className={`surveillance-tile surveillance-tile--${tone}${
        isEmpty ? " surveillance-tile--empty" : ""
      }`}
      aria-labelledby={headingId}
      aria-busy={loading || undefined}
    >
      <div className="surveillance-tile__header">
        <h3 id={headingId} className="surveillance-tile__title">
          {title}
          <span className="surveillance-tile__sr-only">: {countLabel}</span>
        </h3>
        <span className="surveillance-tile__count" aria-hidden="true">
          {loading ? "…" : total === 0 ? "—" : total}
        </span>
      </div>

      {loading ? (
        <p className="surveillance-tile__empty">Loading…</p>
      ) : isEmpty ? (
        <p className="surveillance-tile__empty">No data</p>
      ) : (
        <>
          <ul className="surveillance-tile__list">
            {items.map((item, index) => (
              <li key={`${title}-${item.label}-${index}`} className="surveillance-tile__list-item">
                {item.onClick ? (
                  <button
                    type="button"
                    className="surveillance-tile__button"
                    onClick={item.onClick}
                  >
                    {item.label}
                  </button>
                ) : item.href ? (
                  <a href={item.href} className="surveillance-tile__link">
                    {item.label}
                  </a>
                ) : (
                  <span className="surveillance-tile__text">{item.label}</span>
                )}
              </li>
            ))}
          </ul>
          {hiddenCount > 0 ? <p className="surveillance-tile__more">+{hiddenCount} more</p> : null}
        </>
      )}
    </section>
  );
}

const defaultImmediateAction: SurveillanceTile = {
  title: "Immediate Action",
  items: [],
};

const defaultTakeAction: SurveillanceTile = {
  title: "Take Action",
  items: [],
};

const defaultAlert: SurveillanceTile = {
  title: "Alert",
  items: [],
};

const defaultWatch: SurveillanceTile = {
  title: "Watch",
  items: [],
};

export default function SurveillanceTiles({
  immediateAction = defaultImmediateAction,
  takeAction = defaultTakeAction,
  alert = defaultAlert,
  watch = defaultWatch,
  loading = false,
}: SurveillanceTilesProps) {
  return (
    <div className="surveillance-tiles">
      <div className="surveillance-tiles__grid">
        <TileList {...immediateAction} tone="immediate-action" loading={loading} />
        <TileList {...takeAction} tone="take-action" loading={loading} />
        <TileList {...alert} tone="alert" loading={loading} />
        <TileList {...watch} tone="watch" loading={loading} />
      </div>
    </div>
  );
}
