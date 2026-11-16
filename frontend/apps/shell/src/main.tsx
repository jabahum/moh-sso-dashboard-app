import ReactDOM from "react-dom/client";
import { Provider } from "react-redux";

import App from "./app/App";
import { store } from "@moh-sso/state";
import "leaflet/dist/leaflet.css";

import "./index.scss";
import { AuthBootstrap } from "@moh-sso/auth";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <Provider store={store}>
    <AuthBootstrap />
    <App />
  </Provider>,
);
