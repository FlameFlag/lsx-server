<script lang="ts">
import { onMount } from "svelte";
import type { ProjectData } from "../types";
import ProjectShell from "./ProjectShell.svelte";

export let data: ProjectData;

$: server = data.serverAddr || "127.0.0.1";

let regName = "";
let requestedName = "";
let activationKey = "";
let keyFormat = "";
let keyLoading = false;
let keyError = "";

onMount(() => {
  void fetchKey();
});

async function fetchKey() {
  keyLoading = true;
  keyError = "";
  try {
    const query = requestedName.trim() ? `?name=${encodeURIComponent(requestedName.trim())}` : "";
    const resp = await fetch(`/api/v1/keygen${query}`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const json = await resp.json();
    regName = json.registration_name;
    if (!requestedName.trim()) requestedName = json.registration_name;
    activationKey = json.activation_key;
    keyFormat = json.key_format;
  } catch (e) {
    keyError = e instanceof Error ? e.message : "Failed to generate key";
    regName = "";
    activationKey = "";
    keyFormat = "";
  } finally {
    keyLoading = false;
  }
}
</script>

<ProjectShell active="activate" heading={data.heading || "Activate"}>
  <article class="content-card activate-page">
    <header class="card-header">
      <section>
        <p class="eyebrow">Registration and routing</p>
        <h2 class="compact-heading">Generate a local REGISTER key and route LSX traffic</h2>
        <p class="docs-copy">
          Entering a generated name/key in the <strong>REGISTER</strong> dialog is normally a local Armadillo ShortV3
          check. The same protected runtime also contains legacy Digital River SOAP paths for key issue, activation, and
          validation; the hosts setup below catches those if this install/configuration tries to use them.
        </p>
      </section>
      <a class="sort-link" href="/api/v1/keygen" target="_blank" rel="noopener">Keygen JSON</a>
    </header>

    <dl class="summary-grid" aria-label="Activation requirements">
      <section class="docs-stat">
        <dt>Server IP</dt>
        <dd>{server}</dd>
      </section>
      <section class="docs-stat">
        <dt>Required port</dt>
        <dd>80 for LSX</dd>
      </section>
      <section class="docs-stat">
        <dt>Key format</dt>
        <dd>ShortV3</dd>
      </section>
    </dl>

    <section class="activate-grid" aria-label="Activation tools and checklist">
      <section class="activate-card keygen-panel">
        <p class="eyebrow">Step 1</p>
        <h3>Generate a REGISTER key</h3>
        <p>
          The generated key is bound to the registration name. Armadillo normalizes case and whitespace, but changing
          the actual name text or pairing the key with a different name fails before any network request is made.
        </p>

        <div class="keygen-form">
          <label for="registration-name-input">Registration name</label>
          <input
            id="registration-name-input"
            bind:value={requestedName}
            placeholder="Leave blank for a random name"
            autocomplete="off"
          >
          <button class="game-button" type="button" onclick={fetchKey} disabled={keyLoading}>
            {keyLoading ? "Generating..." : requestedName.trim() ? "Generate for Name" : "Generate Random Pair"}
          </button>
        </div>

        {#if keyError}
          <p class="status-message error">{keyError}</p>
        {/if}

        {#if activationKey}
          <dl class="keygen-output" aria-label="Generated key pair">
            <section>
              <dt>Registration name</dt>
              <dd><code>{regName}</code></dd>
            </section>
            <section>
              <dt>Activation key</dt>
              <dd><code class="activation-key">{activationKey}</code></dd>
            </section>
            <section>
              <dt>Format</dt>
              <dd>{keyFormat}</dd>
            </section>
          </dl>
        {/if}
      </section>

      <section class="activate-card">
        <p class="eyebrow">Step 2</p>
        <h3>Open the local registration dialog</h3>
        <p>
          Run the game with the <code>REGISTER</code> argument and enter the generated name/key pair.
          <strong>Key Valid</strong>
          means Armadillo accepted and stored the ShortV3 key locally.
          <strong>Key Invalid</strong>
          means the signature/name/seed check failed before any LSX traffic is involved. If the dialog chooses Customer
          Service, Buy Now, reissue, or blank-key activation paths, Armadillo may instead send SOAP to the Digital River
          URL stored in its protected configuration.
        </p>
        <pre
          class="copy-block"
        >cd /d "C:\Program Files (x86)\Lemonade Tycoon 2 - New York City" Lemonade2.exe REGISTER</pre>
      </section>
    </section>

    <section class="activate-section">
      <p class="eyebrow">Step 3</p>
      <h3>Route the original services to this server</h3>
      <p>
        Do this for LSX score sync/account creation and for Armadillo&apos;s recovered SOAP operations:
        <code>generateKey</code>, <code>reissueKey</code>, <code>generateKeyForNoTrial</code>,
        <code>validateLicense</code>, and <code>activateLicense</code>. The hosts file can redirect hostnames, but it
        cannot redirect ports, so the game machine must reach this server on TCP port <code>80</code>.
      </p>
      <pre class="copy-block"># Lemonade Tycoon 2: LSX and Armadillo activation
        {server}  gt.jamdat.ca
        {server}  activate.digitalriver.com
        {server}  swreg.org
        {server}  activation.digitalriver.com
      </pre>
      <p class="muted">
        Edit <code>C:\Windows\System32\drivers\etc\hosts</code> as Administrator, save, then run
        <code>ipconfig /flushdns</code>. Confirm from the game machine that
        <code>http://{server}/healthz</code>
        returns <code>OK</code>.
      </p>
    </section>

    <section class="activate-section">
      <p class="eyebrow">What the server answers</p>
      <div class="response-grid">
        <section class="response-card">
          <h4>Account creation</h4>
          <p><code>gt.jamdat.ca/createaccount.php</code></p>
          <strong><code>ACCEPT</code></strong>
        </section>
        <section class="response-card">
          <h4>Score upload</h4>
          <p><code>gt.jamdat.ca/syncgame.php?game=lemonade2</code></p>
          <strong><code>SUCCESS</code></strong>
        </section>
        <section class="response-card">
          <h4>LSX pages</h4>
          <p><code>/lsx2.php</code> and recovered score detail frames</p>
          <strong>Recovered leaderboard HTML</strong>
        </section>
        <section class="response-card">
          <h4>DRM activation</h4>
          <p>Legacy XML/SOAP <code>POST</code> traffic to the Digital River activation host/path</p>
          <strong><code>&lt;result&gt;0&lt;/result&gt;</code> plus key fields when required</strong>
        </section>
      </div>
    </section>

    <section class="activate-section">
      <p class="eyebrow">Recovered Armadillo chain</p>
      <h3>When the protected runtime goes online</h3>
      <p>
        Ghidra mapping shows <code>REGISTER</code> can dispatch <code>generateKey</code> or
        <code>reissueKey</code>
        after the customer-service/manual fallback UI. Startup license-state handling can dispatch
        <code>generateKeyForNoTrial</code>
        when the protected config has the online flags set and local no-trial state is incomplete.
        <code>activateLicense</code> is exposed through an Armadillo API helper.
        <code>validateLicense</code>
        exists and checks stored verification-day/retry state, but this recovered DLL has no static call reference to
        that routine, so it is handled defensively rather than assumed to run every launch.
      </p>
    </section>

    <section class="activate-grid" aria-label="Troubleshooting notes">
      <section class="activate-card">
        <p class="eyebrow">Troubleshooting</p>
        <h3>If REGISTER says Key Invalid</h3>
        <p>
          Generate a fresh pair and copy both fields. The local verifier checks the ShortV3 signature, decrypts the
          payload with a name-derived stream, and checks the embedded license seed. This path does not make an HTTP
          request.
        </p>
      </section>

      <section class="activate-card">
        <p class="eyebrow">Troubleshooting</p>
        <h3>If online features still hit the old servers</h3>
        <p>
          Re-check the hosts entries for the Digital River host actually present in the protected configuration, flush
          DNS, and verify port <code>80</code>. If this Windows install already stored a bad/expired license state,
          remove only this game&apos;s Jamdat or Armadillo registry values after exporting a backup.
        </p>
      </section>
    </section>

    <footer class="activate-footer">
      Unofficial fan preservation project. Not affiliated with EA, JAMDAT, or Digital River.
    </footer>
  </article>
</ProjectShell>
