declare module "opensheetmusicdisplay" {
  interface OSMDOptions {
    autoResize?: boolean;
    backend?: "svg" | "canvas";
    drawTitle?: boolean;
    drawingParameters?: string;
    pageFormat?: string;
  }
  class OpenSheetMusicDisplay {
    constructor(container: HTMLElement | string, options?: OSMDOptions);
    load(url: string): Promise<void>;
    render(): void;
    clear(): void;
  }
  class MusicSheetReadingException extends Error {}
  export { OpenSheetMusicDisplay, MusicSheetReadingException, OSMDOptions };
}
