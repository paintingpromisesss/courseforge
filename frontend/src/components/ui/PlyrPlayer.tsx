import { useEffect, useRef, useMemo } from 'react';
import Plyr from 'plyr';
import 'plyr/dist/plyr.css';
import type { VideoSource } from '../../api/types';

interface Props {
  sources?: VideoSource[];
  src?: string;
}

export function PlyrPlayer({ sources, src }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  const normalizedSources = useMemo<VideoSource[]>(() => {
    if (sources && sources.length > 0) {
      return sources.filter((s) => !!s.src);
    }
    if (src) {
      return [{ src, size: 1080 }];
    }
    return [];
  }, [sources, src]);

  const qualityOptions = useMemo(() => {
    const sizes = normalizedSources
      .map((s) => s.size)
      .filter((s): s is number => typeof s === 'number' && s > 0);
    return Array.from(new Set(sizes)).sort((a, b) => b - a);
  }, [normalizedSources]);

  const defaultQuality = qualityOptions[0] ?? 1080;

  useEffect(() => {
    const container = containerRef.current;
    if (!container || normalizedSources.length === 0) return;

    // Reset container contents
    container.innerHTML = '';

    const video = document.createElement('video');
    video.playsInline = true;
    video.controls = true;
    video.className = 'w-full h-full';

    for (const s of normalizedSources) {
      const source = document.createElement('source');
      source.src = s.src;
      source.type = 'video/mp4';
      if (s.size) {
        source.setAttribute('size', String(s.size));
      }
      video.appendChild(source);
    }

    container.appendChild(video);

    const player = new Plyr(video, {
      iconUrl: '/plyr.svg',
      controls: [
        'play-large',
        'play',
        'progress',
        'current-time',
        'duration',
        'mute',
        'volume',
        'settings',
        'pip',
        'airplay',
        'fullscreen',
      ],
      settings: ['quality', 'speed'],
      speed: {
        selected: 1,
        options: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2],
      },
      quality: {
        default: defaultQuality,
        options: qualityOptions,
        forced: false,
      },
      i18n: {
        qualityLabel: {
          2160: '2160p',
          1440: '1440p',
          1080: '1080p',
          720: '720p',
          480: '480p',
          360: '360p',
          240: '240p',
        },
        speed: 'Скорость',
        normal: 'Обычная',
        quality: 'Качество',
        restart: 'Начать сначала',
        rewind: 'Назад на {seektime} сек',
        play: 'Воспроизвести',
        pause: 'Пауза',
        forward: 'Вперед на {seektime} сек',
        played: 'Воспроизведено',
        buffered: 'Забуферизовано',
        currentTime: 'Текущее время',
        duration: 'Длительность',
        volume: 'Громкость',
        mute: 'Выключить звук',
        unmute: 'Включить звук',
        enterFullscreen: 'Во весь экран',
        exitFullscreen: 'Выйти из полноэкранного режима',
        settings: 'Настройки',
        pip: 'Картинка в картинке',
      },
    });

    return () => {
      try {
        player.destroy();
      } catch {
        // ignore destroy errors
      }
      if (container) {
        container.innerHTML = '';
      }
    };
  }, [normalizedSources, qualityOptions, defaultQuality]);

  if (normalizedSources.length === 0) return null;

  return (
    <div className="my-4 rounded-lg overflow-hidden bg-black aspect-video shadow-md">
      <div ref={containerRef} className="w-full h-full" />
    </div>
  );
}
