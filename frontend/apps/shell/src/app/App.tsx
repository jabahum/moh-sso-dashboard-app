import { ModalProvider, MohThemeProvider, ToastProvider } from "@moh-sso/ui";
import { PwaLifecycle } from "./pwa/PwaLifecycle";
import AppRouter from "./routes/AppRouter";
import { APP_CONFIG } from "../config";
import { StandaloneRouter } from "./routes/StandaloneRouter";

function App() {
  return (
    <MohThemeProvider theme="white">
      <ToastProvider>
        <ModalProvider>
          {!APP_CONFIG.disablePwa && <PwaLifecycle />}
          {APP_CONFIG.standaloneModule ? <StandaloneRouter /> : <AppRouter />}
        </ModalProvider>
      </ToastProvider>
    </MohThemeProvider>
  );
}

export default App;
