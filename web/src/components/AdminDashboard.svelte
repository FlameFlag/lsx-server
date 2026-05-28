<script lang="ts">
  import type { AccountRequest, AdminData, AdminEvent, AdminSubmission } from "../types";

  export let data: AdminData;

  const money = (cents: number) => (cents / 100).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  });

  const dateTime = (value: string) => {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    return date.toLocaleString();
  };

  $: submissions = data.Submissions || [];
  $: accounts = data.Accounts || [];
  $: events = data.Events || [];
  $: lastUpload = data.Stats.LastSubmission ? dateTime(data.Stats.LastSubmission) : "";

  const sectionTitle = (active: AdminData["Active"]) => {
    if (active === "accounts") return "Accounts";
    if (active === "events") return "Request Logs";
    return "Submissions";
  };
</script>

<main class="admin-screen">
  <span class="screen-map" aria-hidden="true"></span>
  <span class="screen-wash" aria-hidden="true"></span>

  <section class="admin-shell">
    <header class="brand-header admin-brand-header">
      <a class="brand-mark admin-brand-mark" href={data.AdminPath}>
        <img class="brand-logo" src="/project/asset/lt2_logo_text_only.avif" alt="Lemonade Tycoon 2 New York Edition" />
      </a>
      <nav class="admin-nav" aria-label="Admin sections">
        <a class:active-tab={data.Active === "submissions"} class="project-tab" href={`${data.AdminPath}/submissions`}>
          <span>Submissions</span>
          <img src="/project/asset/lt2_icon_lsx.avif" alt="" />
        </a>
        <a class:active-tab={data.Active === "accounts"} class="project-tab" href={`${data.AdminPath}/accounts`}>
          <span>Accounts</span>
          <img src="/project/asset/lt2_icon_play.avif" alt="" />
        </a>
        <a class:active-tab={data.Active === "events"} class="project-tab" href={`${data.AdminPath}/events`}>
          <span>Request Logs</span>
          <img src="/project/asset/lt2_icon_findings.avif" alt="" />
        </a>
        <form method="post" action={`${data.AdminPath}/logout`}>
          <button class="project-tab w-full" type="submit">
            <span>Quit</span>
            <img src="/project/asset/lt2_lemon_pair.avif" alt="" />
          </button>
        </form>
      </nav>
    </header>

    <section class="admin-layout">
      <article class="admin-panel">
        <h1 class="panel-title">{sectionTitle(data.Active)}</h1>

        <section class="admin-panel-content">
          {#if data.Flash}
            <p class="flash-message">{data.Flash}</p>
          {/if}

          {#if data.Active === "submissions"}
            {@render SubmissionsSection(data, submissions, money)}
          {:else if data.Active === "accounts"}
            {@render AccountsSection(data, accounts, dateTime)}
          {:else}
            {@render EventsSection(data, events, dateTime)}
          {/if}
        </section>
      </article>

      <aside class="admin-sidebar">
        <dl class="admin-stats">
          <section class="admin-stat"><dt>Submissions</dt><dd>{data.Stats.Submissions}</dd></section>
          <section class="admin-stat"><dt>Accounts</dt><dd>{data.Stats.Accounts}</dd></section>
          <section class="admin-stat"><dt>Request logs</dt><dd>{data.Stats.Events}</dd></section>
          <section class="admin-stat"><dt>Last upload</dt><dd>{lastUpload}</dd></section>
        </dl>
      </aside>
    </section>
  </section>
</main>

{#snippet ActionHidden(action: string, back: string)}
  <input type="hidden" name="csrf" value={data.CSRF} />
  <input type="hidden" name="action" value={action} />
  <input type="hidden" name="back" value={back} />
{/snippet}

{#snippet DeleteButton(action: string, back: string, id: number)}
  <form class="inline" method="post" action={`${data.AdminPath}/action`}>
    {@render ActionHidden(action, back)}
    <input type="hidden" name="id" value={id} />
    <button class="danger-button" type="submit">Delete</button>
  </form>
{/snippet}

{#snippet SubmissionsSection(data: AdminData, submissions: AdminSubmission[], money: (cents: number) => string)}
  <form method="post" action={`${data.AdminPath}/action`}>
    {@render ActionHidden("seed", `${data.AdminPath}/submissions`)}
    <button class="game-button" type="submit">
      <span>Seed recovered Wayback rows</span>
      <img src="/project/asset/lt2_icon_lsx.avif" alt="" />
    </button>
  </form>

  <section class="table-shell">
    <table class="admin-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Company</th>
          <th>Market</th>
          <th>Checksum</th>
          <th>Delete</th>
        </tr>
      </thead>
      <tbody>
        {#each submissions as sub}
          <tr>
            <td class="whitespace-nowrap">#{sub.ID}<br /><span class="muted">{sub.When}</span><br /><span class="muted">{sub.Remote}</span></td>
            <td><b>{sub.Row.Company}</b><br />{sub.Row.CEO}<br /><span class="muted">{sub.Host}</span></td>
            <td class="font-black">${money(sub.Row.MarketCents)}</td>
            <td>
              {#if sub.Valid}
                <b class="valid-state">valid</b>
              {:else}
                <b class="invalid-state"><img src="/project/asset/warning.avif" alt="" />mismatch</b>
              {/if}
              <br /><span class="muted">client {sub.Client}</span><br /><span class="muted">computed {sub.Computed}</span>
            </td>
            <td>{@render DeleteButton("delete_submission", `${data.AdminPath}/submissions`, sub.ID)}</td>
          </tr>
          <tr>
            <td colspan="5">
              <details class="edit-panel">
                <summary>Edit fields</summary>
                <form class="edit-form" method="post" action={`${data.AdminPath}/action`}>
                  {@render ActionHidden("update_submission", `${data.AdminPath}/submissions`)}
                  <input type="hidden" name="id" value={sub.ID} />
                  <fieldset class="field-grid">
                    {#each data.Fields as field}
                      <label>
                        <span>{field}</span>
                        <input class="admin-input" name={field} value={sub.Fields[field] || ""} />
                      </label>
                    {/each}
                  </fieldset>
                  <button class="save-button" type="submit">Save submission</button>
                </form>
              </details>
            </td>
          </tr>
        {:else}
          <tr><td colspan="5">No submissions yet.</td></tr>
        {/each}
      </tbody>
    </table>
  </section>
{/snippet}

{#snippet AccountsSection(data: AdminData, accounts: AccountRequest[], dateTime: (value: string) => string)}
  <section class="table-shell">
    <table class="admin-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Received</th>
          <th>Remote</th>
          <th>Username</th>
          <th>Password</th>
          <th>Raw query</th>
          <th>Delete</th>
        </tr>
      </thead>
      <tbody>
        {#each accounts as account}
          <tr>
            <td>#{account.ID}</td>
            <td>{dateTime(account.ReceivedAt)}</td>
            <td>{account.RemoteAddr}</td>
            <td><b>{account.Username}</b></td>
            <td>{account.Password}</td>
            <td class="raw-query">{account.RawQuery}</td>
            <td>{@render DeleteButton("delete_account", `${data.AdminPath}/accounts`, account.ID)}</td>
          </tr>
        {:else}
          <tr><td colspan="7">No account probes yet.</td></tr>
        {/each}
      </tbody>
    </table>
  </section>
{/snippet}

{#snippet EventsSection(data: AdminData, events: AdminEvent[], dateTime: (value: string) => string)}
  <form method="post" action={`${data.AdminPath}/action`}>
    {@render ActionHidden("clear_events", `${data.AdminPath}/events`)}
    <button class="danger-button" type="submit">Clear all request logs</button>
  </form>

  <section class="table-shell">
    <table class="admin-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Time</th>
          <th>Kind</th>
          <th>Status</th>
          <th>Remote</th>
          <th>Path</th>
          <th>Message</th>
          <th>Delete</th>
        </tr>
      </thead>
      <tbody>
        {#each events as event}
          <tr>
            <td>#{event.ID}</td>
            <td>{dateTime(event.Time)}</td>
            <td>{event.Kind}</td>
            <td>{event.Status}</td>
            <td>{event.RemoteAddr}</td>
            <td>
              <b>{event.Route}</b>
              {#if event.Summary?.length}
                <p class="event-summary">
                  {#each event.Summary as item}
                    <span><b>{item.Label}</b> {item.Value}</span>
                  {/each}
                </p>
              {/if}
              <details class="raw-request">
                <summary>Raw request</summary>
                <code>{event.Raw}</code>
              </details>
            </td>
            <td>{event.Message}</td>
            <td>{@render DeleteButton("delete_event", `${data.AdminPath}/events`, event.ID)}</td>
          </tr>
        {:else}
          <tr><td colspan="8">No request logs yet.</td></tr>
        {/each}
      </tbody>
    </table>
  </section>
{/snippet}
