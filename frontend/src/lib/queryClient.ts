import { QueryClient } from "@tanstack/react-query";

// App・testごとに独立したserver state cacheを生成する。
export function createAppQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
}
