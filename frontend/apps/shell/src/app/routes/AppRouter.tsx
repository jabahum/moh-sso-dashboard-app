import { BrowserRouter as Router, Navigate, Route, Routes } from "react-router-dom";

import { adminRoutes } from "./admin.routes";
import { publicRoutes } from "./public.routes";
import { userRoutes } from "./user.routes";

function AppRouter() {
  return (
    <Router>
      <Routes>
        {publicRoutes}
        {userRoutes}
        {adminRoutes}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Router>
  );
}

export default AppRouter;
