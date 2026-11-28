import { PageLoading } from "./PageLoading";

export default {
  title: "Feedback/PageLoading",
};

export const Default = () => <PageLoading />;

export const WithDescription = () => <PageLoading description="Loading users…" />;
