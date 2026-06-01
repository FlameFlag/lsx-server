import "./app.css";
import { type Component, mount } from "svelte";
import type { PageKind } from "./types";

const target = document.getElementById("app");

if (!target) {
  throw new Error("Svelte mount target #app was not found.");
}

const page = (target.dataset.page || "home") as PageKind;
const dataNode = document.getElementById("lsx-page-data");
const data = JSON.parse(dataNode?.textContent || target.dataset.initial || "{}") as unknown;

document.documentElement.style.setProperty("--lsx-menu-map-backdrop", "url('/project/asset/menu_map_backdrop.avif')");
document.documentElement.style.setProperty("--lsx-green-pill", "url('/project/asset/lt2_green_pill.avif')");

const PageComponent = await componentFor(page);

mount(PageComponent, {
  target,
  props: { data }
});

async function componentFor(page: PageKind): Promise<Component<{ data: unknown }>> {
  if (page === "admin")
    return (await import("./components/AdminDashboard.svelte")).default as Component<{ data: unknown }>;
  if (page === "admin-login")
    return (await import("./components/AdminLogin.svelte")).default as Component<{ data: unknown }>;
  if (page === "findings")
    return (await import("./components/FindingsPage.svelte")).default as Component<{ data: unknown }>;
  if (page === "activate")
    return (await import("./components/ActivatePage.svelte")).default as Component<{ data: unknown }>;
  if (page === "docs") return (await import("./components/DocsPage.svelte")).default as Component<{ data: unknown }>;
  return (await import("./components/HomePage.svelte")).default as Component<{ data: unknown }>;
}
