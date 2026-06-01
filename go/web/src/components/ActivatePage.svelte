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
      <p class="eyebrow">ShortV3 math</p>
      <h3>How the key is actually built</h3>
      <p>
        The key is a signed, name-bound ShortV3 payload. The game does not just compare a string. It decodes the key,
        decrypts a small payload with a stream derived from the registration name, hashes that payload together with the
        same cooked name, then verifies an ElGamal-style signature against the public certificate embedded in the
        protected data.
      </p>

      <div class="math-flow" aria-label="ShortV3 keygen math">
        <section class="math-card">
          <h4>1. Cook the name</h4>
          <p>
            The verifier first normalizes the registration name by deleting spaces, tabs, CR/LF, and uppercasing ASCII
            letters. That cooked name is used twice: once to derive the payload mask and once inside the signed message.
            This is why the displayed name/key pair must stay together.
          </p>
          <pre class="copy-block">cookText("Jane Doe") = "JANEDOE"</pre>
        </section>

        <section class="math-card">
          <h4>2. Pack the payload</h4>
          <p>
            The unsigned payload is six bytes: a 16-bit Armadillo day count, followed by the recovered level-25 license
            seed. The day count is days since <code>1999-01-01</code>. The seed is <code>0xCCF0580A</code>; the
            generator writes bytes so the target's big-endian seed read sees exactly that dword.
          </p>
          <pre class="copy-block">payload = LE16(today) || CC F0 58 0A</pre>
        </section>

        <section class="math-card">
          <h4>3. Hide it with the name stream</h4>
          <p>
            The cooked name is CRC32-hashed with Armadillo's reflected CRC table. That CRC seeds the AKT PRNG:
            <code>state = (state * 31415821 + 1) mod 100000000</code>. Each payload byte is XORed with the next
            generated byte. A different name produces a different stream, so the same signature cannot be reused for
            another visible name.
          </p>
          <pre class="copy-block">masked[i] = payload[i] xor nextRange(256)</pre>
        </section>

        <section class="math-card">
          <h4>4. Sign the payload and name</h4>
          <p>
            ShortV3 level 25 uses a 9-byte group. The modulus is <code>p = 2^72 + 0xE3B</code>, the generator is
            <code>0xF3C7E00A4B58155299</code>, and the public certificate is <code>0x9CC50E4D25416464B9</code>. The
            private exponent recovered for this project is <code>0x70301169DE7C75D66F</code>, satisfying
            <code>public = generator^private mod p</code>.
          </p>
          <pre class="copy-block">message = MD5(maskedPayload || cookText(name))</pre>
        </section>

        <section class="math-card">
          <h4>5. Make the ElGamal pair</h4>
          <p>
            A random nonce <code>k</code> is derived from five CRC32 passes and must be invertible modulo
            <code>p - 1</code>. The signature parts are <code>a = generator^k mod p</code> and
            <code>b = (message - private * a) * inverse(k) mod (p - 1)</code>. The verifier checks the matching public
            equation, which proves the key was produced with the private exponent without storing that exponent in the
            game.
          </p>
          <pre class="copy-block">generator^message == public^a * a^b  (mod p)</pre>
        </section>

        <section class="math-card">
          <h4>6. Encode the final key</h4>
          <p>
            The final byte string is the masked six-byte payload plus interleaved little-endian bytes of
            <code>a</code>
            and <code>b</code>. It is converted to Armadillo's 32-character alphabet
            <code>0123456789ABCDEFGHJKMNPQRTUVWXYZ</code>, grouped every six characters, with the top digit marked so
            the decoder knows where the big integer ends.
          </p>
        </section>
      </div>
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
