declare module "redoc/bundles/redoc.standalone.js?url" {
  const url: string;
  export default url;
}

type RedocApi = {
  init: (specUrl: string, options: Record<string, unknown>, element: HTMLElement) => void;
};

interface Window {
  Redoc: RedocApi;
}
