<script lang="ts">
  import AdminDashboard from "./components/AdminDashboard.svelte";
  import AdminLogin from "./components/AdminLogin.svelte";
  import DocsPage from "./components/DocsPage.svelte";
  import FindingsPage from "./components/FindingsPage.svelte";
  import HomePage from "./components/HomePage.svelte";
  import type { AdminData, AdminLoginData, PageKind, ProjectData } from "./types";

  export let page: PageKind;
  export let data: unknown;

  $: projectData = data as ProjectData;
  $: adminData = data as AdminData;
  $: adminLoginData = data as AdminLoginData;
</script>

{#if page === "admin"}
  <AdminDashboard data={adminData} />
{:else if page === "admin-login"}
  <AdminLogin data={adminLoginData} />
{:else if page === "findings"}
  <FindingsPage data={projectData} />
{:else if page === "docs"}
  <DocsPage data={projectData} />
{:else}
  <HomePage data={projectData} />
{/if}
