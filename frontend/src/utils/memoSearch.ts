export type SearchScope = 'text' | 'all';

export interface MemoItem {
  id: string;
  type?: 'text' | 'voice' | 'audio';
  title?: string;
  content?: string;
  transcript?: string;
  tags?: string[];
  createdAt?: string;
  created_at?: string;
  color?: string;
  duration?: string;
  project?: string;
  workspace_path?: string;
  [key: string]: any;
}

/**
 * Filters a list of memos dynamically in real time across title, content, tags, and transcript.
 *
 * @param memos The list of memos to filter.
 * @param query The search query string.
 * @param searchScope The search scope: 'text' (default, searches text memos only) or 'all' (searches text and voice memos).
 * @returns The filtered array of memos matching the query.
 */
export function filterMemos(
  memos: MemoItem[],
  query: string,
  searchScope: SearchScope = 'text'
): MemoItem[] {
  if (!Array.isArray(memos)) {
    return [];
  }

  const trimmedQuery = query ? query.trim() : '';
  if (!trimmedQuery) {
    return [...memos];
  }

  const normalizedQuery = trimmedQuery.toLowerCase();
  const effectiveScope: SearchScope = searchScope === 'all' ? 'all' : 'text';

  return memos.filter((memo) => {
    if (!memo) return false;

    const isVoice =
      memo.type === 'voice' ||
      memo.type === 'audio' ||
      (Array.isArray(memo.tags) && memo.tags.some((t) => typeof t === 'string' && t.toLowerCase() === 'voice') && memo.type !== 'text');

    // When searchScope is 'text', exclude voice memos from matching search queries
    if (effectiveScope === 'text' && isVoice) {
      return false;
    }

    // Match keywords against title, content, transcript, and tags
    const titleMatch = typeof memo.title === 'string' && memo.title.toLowerCase().includes(normalizedQuery);
    const contentMatch = typeof memo.content === 'string' && memo.content.toLowerCase().includes(normalizedQuery);
    const transcriptMatch = typeof memo.transcript === 'string' && memo.transcript.toLowerCase().includes(normalizedQuery);
    const tagsMatch =
      Array.isArray(memo.tags) &&
      memo.tags.some((tag) => typeof tag === 'string' && tag.toLowerCase().includes(normalizedQuery));

    return titleMatch || contentMatch || transcriptMatch || tagsMatch;
  });
}
