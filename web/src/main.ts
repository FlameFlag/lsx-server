import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";
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

mount(App, {
  target,
  props: { page, data }
});
