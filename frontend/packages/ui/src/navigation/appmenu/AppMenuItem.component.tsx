import React from "react";
import { ClickableTile } from "@carbon/react";
import { Launch } from "@carbon/react/icons";

import "./AppMenu.scss";

interface AppTileProps {
  icon: React.ElementType;
  name: string;
  description?: string;
  href: string;
  clientId?: string;
  onSelect?: () => void;
  onOpen?: (href: string, clientId?: string) => void;
  launchMode?: "internal" | "new_tab" | "same_tab";
}

function isExternalUrl(url: string): boolean {
  return /^https?:\/\//i.test(url);
}

function isValidHref(href?: string | null): href is string {
  return Boolean(href && href.trim().length > 0 && href.trim() !== "#");
}

function getTileId(name: string) {
  return `app-menu-tile-${name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "")}`;
}

const AppTile: React.FC<AppTileProps> = ({
  icon: Icon,
  name,
  description,
  href,
  clientId,
  onSelect,
  onOpen,
  launchMode = "internal",
}) => {
  const disabled = !isValidHref(href);
  const external = isValidHref(href) && isExternalUrl(href);
  const opensNewTab = external && launchMode === "new_tab";

  const handleClick = (event: React.SyntheticEvent<HTMLElement>) => {
    if (onOpen || disabled) {
      event.preventDefault();
    }
    if (disabled) {
      return;
    }

    onSelect?.();

    onOpen?.(href, clientId);
  };

  return (
    <ClickableTile
      href={disabled ? undefined : href}
      id={getTileId(name)}
      className={["app-menu-tile", disabled ? "app-menu-tile--disabled" : ""]
        .filter(Boolean)
        .join(" ")}
      onClick={handleClick}
      disabled={disabled}
      aria-label={opensNewTab ? `Open ${name} in a new tab` : `Open ${name}`}
    >
      <span className="app-menu-tile__icon-wrap" aria-hidden="true">
        <Icon size={24} className="app-menu-tile__icon" />
      </span>

      <span className="app-menu-tile__label">{name}</span>
      {description ? <span className="app-menu-tile__description">{description}</span> : null}

      {opensNewTab && (
        <span className="app-menu-tile__external" aria-hidden="true">
          <Launch size={14} />
        </span>
      )}
    </ClickableTile>
  );
};

export default AppTile;
