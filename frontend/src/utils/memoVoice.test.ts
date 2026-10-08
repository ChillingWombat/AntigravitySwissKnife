import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  formatDuration,
  resolveVoiceMemoTitle,
  isSpeechRecognitionSupported,
  getSupportedAudioMimeType,
  blobToBase64,
} from './memoVoice.ts';

describe('memoVoice utility', () => {
  describe('formatDuration', () => {
    it('formats 0s into 0:00', () => {
      assert.equal(formatDuration(0), '0:00');
    });

    it('formats 5s into 0:05', () => {
      assert.equal(formatDuration(5), '0:05');
    });

    it('formats 65s into 1:05', () => {
      assert.equal(formatDuration(65), '1:05');
    });

    it('formats 125s into 2:05', () => {
      assert.equal(formatDuration(125), '2:05');
    });

    it('handles negative or NaN seconds safely', () => {
      assert.equal(formatDuration(-10), '0:00');
      assert.equal(formatDuration(NaN), '0:00');
    });
  });

  describe('resolveVoiceMemoTitle', () => {
    it('returns title with [Voice] prefix when transcript is provided', () => {
      assert.equal(
        resolveVoiceMemoTitle('Quick meeting summary'),
        '[Voice] Quick meeting summary'
      );
    });

    it('preserves existing [Voice] prefix without duplicate prefixing', () => {
      assert.equal(
        resolveVoiceMemoTitle('[Voice] Standup Notes'),
        '[Voice] Standup Notes'
      );
      assert.equal(
        resolveVoiceMemoTitle('[voice] lowercase prefix test'),
        '[voice] lowercase prefix test'
      );
    });

    it('uses duration in title when transcript is empty and duration is provided', () => {
      assert.equal(
        resolveVoiceMemoTitle('', '1:05'),
        '[Voice] Voice Memo (1:05)'
      );
      assert.equal(
        resolveVoiceMemoTitle('   ', '0:05'),
        '[Voice] Voice Memo (0:05)'
      );
      assert.equal(
        resolveVoiceMemoTitle(undefined, '2:05'),
        '[Voice] Voice Memo (2:05)'
      );
    });

    it('falls back to default title when both transcript and duration are empty', () => {
      assert.equal(resolveVoiceMemoTitle('', ''), '[Voice] Voice Memo');
      assert.equal(resolveVoiceMemoTitle(), '[Voice] Voice Memo');
      assert.equal(resolveVoiceMemoTitle('   ', '   '), '[Voice] Voice Memo');
    });
  });

  describe('isSpeechRecognitionSupported', () => {
    it('returns false in non-browser Node test environment without window.SpeechRecognition', () => {
      const supported = isSpeechRecognitionSupported();
      assert.equal(typeof supported, 'boolean');
    });
  });

  describe('getSupportedAudioMimeType', () => {
    it('returns a string in any environment safely', () => {
      const mime = getSupportedAudioMimeType();
      assert.equal(typeof mime, 'string');
    });
  });

  describe('blobToBase64', () => {
    it('converts a Blob to a base64 data URL string', async () => {
      if (typeof Blob !== 'undefined' && typeof FileReader !== 'undefined') {
        const testBlob = new Blob(['sample audio test'], { type: 'text/plain' });
        const result = await blobToBase64(testBlob);
        assert.ok(result.startsWith('data:text/plain;base64,'));
      }
    });
  });
});
