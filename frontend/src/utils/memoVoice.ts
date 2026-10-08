/**
 * memoVoice utility functions for voice memo audio recording and Web Speech API transcription.
 */

/**
 * Checks whether native Web Speech API (SpeechRecognition or webkitSpeechRecognition)
 * is supported in the current browser/Electron runtime environment.
 */
export function isSpeechRecognitionSupported(): boolean {
  if (typeof window === 'undefined') return false;
  return Boolean(
    (window as any).SpeechRecognition ||
    (window as any).webkitSpeechRecognition
  );
}

/**
 * Formats a duration in seconds into a standard mm:ss string (e.g. 0 -> "0:00", 5 -> "0:05", 65 -> "1:05").
 */
export function formatDuration(seconds: number): string {
  const safeSec = Math.max(0, Math.floor(seconds || 0));
  const mins = Math.floor(safeSec / 60);
  const secs = safeSec % 60;
  return `${mins}:${secs < 10 ? '0' : ''}${secs}`;
}

/**
 * Resolves a display title for a voice memo from an optional transcript and duration.
 * Prefixes with [Voice] if not already present.
 */
export function resolveVoiceMemoTitle(transcript?: string, duration?: string): string {
  const cleanTranscript = (transcript || '').trim();
  if (cleanTranscript) {
    if (cleanTranscript.toLowerCase().startsWith('[voice]')) {
      return cleanTranscript;
    }
    return `[Voice] ${cleanTranscript}`;
  }
  const cleanDuration = (duration || '').trim();
  if (cleanDuration) {
    return `[Voice] Voice Memo (${cleanDuration})`;
  }
  return '[Voice] Voice Memo';
}

/**
 * Detects the first supported audio MIME container/codec for MediaRecorder.
 */
export function getSupportedAudioMimeType(): string {
  if (typeof window === 'undefined' || typeof MediaRecorder === 'undefined') {
    return '';
  }
  const candidates = [
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/ogg;codecs=opus',
    'audio/ogg',
    'audio/mp4',
  ];
  for (const mime of candidates) {
    if (typeof MediaRecorder.isTypeSupported === 'function' && MediaRecorder.isTypeSupported(mime)) {
      return mime;
    }
  }
  return '';
}

/**
 * Converts an audio Blob into a Base64 data URL string.
 */
export async function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onloadend = () => {
      resolve(typeof reader.result === 'string' ? reader.result : '');
    };
    reader.onerror = () => {
      resolve('');
    };
    reader.readAsDataURL(blob);
  });
}
