<script lang="ts">
  import { VNCKeyboard, KEYSYM } from "$lib/vnc-keyboard";

  interface Props {
    name: string;
    vncPort: number;
  }

  let { name, vncPort }: Props = $props();
  let expanded = $state(false);
  let pressed = $state<Record<string, boolean>>({});
  let vncConnected = $state(false);

  let vnc: VNCKeyboard | null = null;

  $effect(() => {
    if (expanded) {
      const host = window.location.hostname;
      vnc = new VNCKeyboard(
        `ws://${host}:${vncPort}/websockify`,
        (connected) => { vncConnected = connected; }
      );
      vnc.connect();
    } else {
      vnc?.disconnect();
      vnc = null;
      vncConnected = false;
    }
    return () => {
      vnc?.disconnect();
      vnc = null;
    };
  });

  type ButtonDef = { label: string; key: string };

  const faceButtons: ButtonDef[] = [
    { label: "A", key: "a" },
    { label: "B", key: "b" },
    { label: "X", key: "x" },
    { label: "Y", key: "y" },
  ];

  const dpad: ButtonDef[] = [
    { label: "\u25B2", key: "Up" },
    { label: "\u25BC", key: "Down" },
    { label: "\u25C0", key: "Left" },
    { label: "\u25B6", key: "Right" },
  ];

  const leftStick: ButtonDef[] = [
    { label: "E", key: "e" },
    { label: "D", key: "d" },
    { label: "S", key: "s" },
    { label: "F", key: "f" },
  ];

  const rightStick: ButtonDef[] = [
    { label: "I", key: "i" },
    { label: "K", key: "k" },
    { label: "J", key: "j" },
    { label: "L", key: "l" },
  ];

  function handleDown(key: string) {
    pressed[key] = true;
    const sym = KEYSYM[key];
    if (sym != null) vnc?.sendKey(sym, true);
  }

  function handleUp(key: string) {
    pressed[key] = false;
    const sym = KEYSYM[key];
    if (sym != null) vnc?.sendKey(sym, false);
  }

  function onPointerDown(e: PointerEvent, key: string) {
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    handleDown(key);
  }

  function onPointerUp(e: PointerEvent, key: string) {
    e.preventDefault();
    handleUp(key);
  }
</script>

<div class="mt-1">
  <button
    class="text-[11px] text-dim hover:text-text"
    onclick={() => (expanded = !expanded)}
  >
    {expanded ? "Hide Controls" : "Controls"}
    {#if expanded}
      <span class="ml-1 text-[9px]" class:text-hgreen={vncConnected} class:text-hred={!vncConnected}>
        {vncConnected ? "\u25CF" : "\u25CB"}
      </span>
    {/if}
  </button>

  {#if expanded}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="mt-1 select-none rounded border border-border bg-card p-3"
      ontouchstart={(e) => e.stopPropagation()}
    >
      <!-- Top bar: LB LT Back Start RT RB -->
      <div class="mb-2 flex items-center justify-between gap-1">
        <div class="flex gap-1">
          <button
            class="btn-ctrl"
            class:btn-active={pressed["1"]}
            onpointerdown={(e) => onPointerDown(e, "1")}
            onpointerup={(e) => onPointerUp(e, "1")}
            onpointercancel={(e) => onPointerUp(e, "1")}
          >LB</button>
          <button
            class="btn-ctrl"
            class:btn-active={pressed["w"]}
            onpointerdown={(e) => onPointerDown(e, "w")}
            onpointerup={(e) => onPointerUp(e, "w")}
            onpointercancel={(e) => onPointerUp(e, "w")}
          >LT</button>
        </div>
        <div class="flex gap-1">
          <button
            class="btn-ctrl"
            class:btn-active={pressed["BackSpace"]}
            onpointerdown={(e) => onPointerDown(e, "BackSpace")}
            onpointerup={(e) => onPointerUp(e, "BackSpace")}
            onpointercancel={(e) => onPointerUp(e, "BackSpace")}
          >Bk</button>
          <button
            class="btn-ctrl"
            class:btn-active={pressed["Return"]}
            onpointerdown={(e) => onPointerDown(e, "Return")}
            onpointerup={(e) => onPointerUp(e, "Return")}
            onpointercancel={(e) => onPointerUp(e, "Return")}
          >St</button>
        </div>
        <div class="flex gap-1">
          <button
            class="btn-ctrl"
            class:btn-active={pressed["o"]}
            onpointerdown={(e) => onPointerDown(e, "o")}
            onpointerup={(e) => onPointerUp(e, "o")}
            onpointercancel={(e) => onPointerUp(e, "o")}
          >RT</button>
          <button
            class="btn-ctrl"
            class:btn-active={pressed["2"]}
            onpointerdown={(e) => onPointerDown(e, "2")}
            onpointerup={(e) => onPointerUp(e, "2")}
            onpointercancel={(e) => onPointerUp(e, "2")}
          >RB</button>
        </div>
      </div>

      <!-- Middle: D-pad (left) + Face buttons (right) -->
      <div class="mb-2 flex items-center justify-between">
        <!-- D-pad -->
        <div class="grid grid-cols-3 grid-rows-3 gap-0.5" style="width: 120px; height: 120px;">
          <div></div>
          {#each [dpad[0]] as btn}
            <button
              class="btn-dpad"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [dpad[2]] as btn}
            <button
              class="btn-dpad"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [dpad[3]] as btn}
            <button
              class="btn-dpad"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [dpad[1]] as btn}
            <button
              class="btn-dpad"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
        </div>

        <!-- Face buttons (diamond layout) -->
        <div class="grid grid-cols-3 grid-rows-3 gap-0.5" style="width: 120px; height: 120px;">
          <div></div>
          {#each [faceButtons[3]] as btn}
            <button
              class="btn-face btn-face-y"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [faceButtons[2]] as btn}
            <button
              class="btn-face btn-face-x"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [faceButtons[1]] as btn}
            <button
              class="btn-face btn-face-b"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [faceButtons[0]] as btn}
            <button
              class="btn-face btn-face-a"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
        </div>
      </div>

      <!-- Bottom: Left stick + Right stick + L3/R3 -->
      <div class="flex items-center justify-between">
        <!-- Left stick (ESDF) -->
        <div class="grid grid-cols-3 grid-rows-3 gap-0.5" style="width: 108px; height: 108px;">
          <div></div>
          {#each [leftStick[0]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [leftStick[2]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <button
            class="btn-stick text-[9px]"
            class:btn-active={pressed["3"]}
            onpointerdown={(e) => onPointerDown(e, "3")}
            onpointerup={(e) => onPointerUp(e, "3")}
            onpointercancel={(e) => onPointerUp(e, "3")}
          >L3</button>
          {#each [leftStick[3]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [leftStick[1]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
        </div>

        <!-- Right stick (IJKL) -->
        <div class="grid grid-cols-3 grid-rows-3 gap-0.5" style="width: 108px; height: 108px;">
          <div></div>
          {#each [rightStick[0]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [rightStick[2]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <button
            class="btn-stick text-[9px]"
            class:btn-active={pressed["4"]}
            onpointerdown={(e) => onPointerDown(e, "4")}
            onpointerup={(e) => onPointerUp(e, "4")}
            onpointercancel={(e) => onPointerUp(e, "4")}
          >R3</button>
          {#each [rightStick[3]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
          {#each [rightStick[1]] as btn}
            <button
              class="btn-stick"
              class:btn-active={pressed[btn.key]}
              onpointerdown={(e) => onPointerDown(e, btn.key)}
              onpointerup={(e) => onPointerUp(e, btn.key)}
              onpointercancel={(e) => onPointerUp(e, btn.key)}
            >{btn.label}</button>
          {/each}
          <div></div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .btn-ctrl {
    min-width: 36px;
    padding: 6px 8px;
    font-size: 10px;
    font-weight: 700;
    border-radius: 4px;
    border: 1px solid var(--color-border);
    background: var(--color-card2);
    color: var(--color-text);
    touch-action: none;
    user-select: none;
    transition: background 0.05s;
  }

  .btn-dpad {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 700;
    border-radius: 4px;
    border: 1px solid var(--color-border);
    background: var(--color-card2);
    color: var(--color-text);
    touch-action: none;
    user-select: none;
    transition: background 0.05s;
  }

  .btn-face {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 700;
    border-radius: 50%;
    border: 1px solid;
    touch-action: none;
    user-select: none;
    transition: background 0.05s;
  }

  .btn-face-a {
    border-color: color-mix(in srgb, var(--color-hgreen) 40%, transparent);
    background: color-mix(in srgb, var(--color-hgreen) 10%, transparent);
    color: var(--color-hgreen);
  }
  .btn-face-b {
    border-color: color-mix(in srgb, var(--color-hred) 40%, transparent);
    background: color-mix(in srgb, var(--color-hred) 10%, transparent);
    color: var(--color-hred);
  }
  .btn-face-x {
    border-color: color-mix(in srgb, var(--color-hblue) 40%, transparent);
    background: color-mix(in srgb, var(--color-hblue) 10%, transparent);
    color: var(--color-hblue);
  }
  .btn-face-y {
    border-color: color-mix(in srgb, var(--color-hyellow) 40%, transparent);
    background: color-mix(in srgb, var(--color-hyellow) 10%, transparent);
    color: var(--color-hyellow);
  }

  .btn-stick {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    font-weight: 700;
    border-radius: 4px;
    border: 1px solid var(--color-border);
    background: var(--color-card2);
    color: var(--color-dim);
    touch-action: none;
    user-select: none;
    transition: background 0.05s;
  }

  .btn-active {
    background: color-mix(in srgb, var(--color-hgold) 30%, transparent) !important;
    border-color: var(--color-hgold) !important;
    color: var(--color-hgold) !important;
  }
</style>
