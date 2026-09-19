declare module 'plyr' {
  export interface Source {
    src: string;
    type?: string;
    size?: number;
    provider?: 'html5' | 'youtube' | 'vimeo';
  }

  export interface SourceOptions {
    type: 'video' | 'audio';
    title?: string;
    sources: Source[];
    poster?: string;
  }

  export interface Options {
    controls?: string[];
    settings?: string[];
    speed?: { selected?: number; options?: number[] };
    quality?: {
      default?: number;
      options?: number[];
      forced?: boolean;
      onChange?: (quality: number) => void;
    };
    i18n?: Record<string, unknown>;
    ratio?: string;
    seekTime?: number;
    autoplay?: boolean;
    loop?: { active?: boolean };
    keyboard?: { focused?: boolean; global?: boolean };
  }

  export default class Plyr {
    constructor(target: HTMLElement | HTMLMediaElement | string, options?: Options);
    destroy(): void;
    source: SourceOptions | string;
    currentTime: number;
    duration: number;
    paused: boolean;
    playing: boolean;
    play(): Promise<void>;
    pause(): void;
    stop(): void;
    restart(): void;
    on(event: string, callback: (event: CustomEvent) => void): void;
    quality: number;
  }
}
