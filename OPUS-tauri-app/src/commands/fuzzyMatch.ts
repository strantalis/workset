export type FuzzyResult<T> = {
  item: T;
  score: number;
  matches: number[];
};

export function fuzzyMatch(
  query: string,
  label: string,
): { score: number; matches: number[] } | null {
  const q = query.toLowerCase();
  const l = label.toLowerCase();

  if (q.length === 0) return { score: 1, matches: [] };
  if (q.length > l.length) return null;

  const matches: number[] = [];
  let qi = 0;
  let score = 0;
  let prevMatchIndex = -2;

  for (let li = 0; li < l.length && qi < q.length; li++) {
    if (l[li] === q[qi]) {
      matches.push(li);

      // Consecutive match bonus
      if (li === prevMatchIndex + 1) {
        score += 3;
      }

      // Word boundary bonus
      if (li === 0 || /[\s\-/]/.test(l[li - 1])) {
        score += 2;
      }

      // Earlier match bonus (diminishing)
      score += Math.max(0, 10 - li);

      prevMatchIndex = li;
      qi++;
    }
  }

  if (qi < q.length) return null;

  return { score, matches };
}

export function fuzzyFilter<T>(
  items: T[],
  query: string,
  getLabel: (item: T) => string,
): FuzzyResult<T>[] {
  if (query.trim() === '') {
    return items.map((item) => ({ item, score: 0, matches: [] }));
  }

  const results: FuzzyResult<T>[] = [];
  for (const item of items) {
    const match = fuzzyMatch(query, getLabel(item));
    if (match) {
      results.push({ item, score: match.score, matches: match.matches });
    }
  }

  return results.sort((a, b) => b.score - a.score);
}
