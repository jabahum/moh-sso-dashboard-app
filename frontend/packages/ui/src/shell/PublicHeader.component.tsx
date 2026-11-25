import { Login } from "@carbon/react/icons";
import { Header, HeaderName, HeaderGlobalBar, Button, SkipToContent } from "@carbon/react";
import type React from "react";

import "./public-header.scss";
import imagePath from "../assets/logo.png";

type PublicHeaderProps = {
  onNavigateHome?: () => void;
  homeHref?: string;
  onLogin?: () => void;
};

const PublicHeader: React.FC<PublicHeaderProps> = ({
  onNavigateHome,
  onLogin,
  homeHref = "/",
}) => {
  return (
    <Header aria-label="MOH Integrated Health Portal" className="moh-shell-header">
      <SkipToContent />

      <img src={imagePath} className="moh-image-style" alt="Ministry of Health logo" />

      <HeaderName
        prefix="MOH"
        href={homeHref}
        onClick={(event) => {
          if (
            onNavigateHome &&
            !event.metaKey &&
            !event.ctrlKey &&
            !event.shiftKey &&
            !event.altKey
          ) {
            event.preventDefault();
            onNavigateHome();
          }
        }}
        className="moh-header-name"
      >
        Integrated Health Portal
      </HeaderName>

      <HeaderGlobalBar>
        <Button
          kind="ghost"
          size="lg"
          renderIcon={Login}
          className="moh-header-login"
          onClick={onLogin}
        >
          Sign in
        </Button>
      </HeaderGlobalBar>
    </Header>
  );
};

export default PublicHeader;
