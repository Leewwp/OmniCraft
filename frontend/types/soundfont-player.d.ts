declare module "soundfont-player" {
  interface Player {
    play(
      note: number | string | number[],
      time?: number,
      options?: { gain?: number; duration?: number },
    ): void;
    stop(): void;
    on(event: string, cb: (...args: unknown[]) => void): void;
  }
  interface InstrumentOptions {
    gain?: number;
    release?: number;
    destination?: AudioNode;
  }
  function instrument(
    ctx: AudioContext,
    name: string,
    options?: InstrumentOptions,
  ): Promise<Player>;
  function noteToMidi(name: string): number;
  export { instrument, noteToMidi, Player, InstrumentOptions };
}
