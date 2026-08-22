type PreviewState = 'setupRequired' | 'ready' | 'error';

export function installDevPreview() {
    if (!import.meta.env.DEV && import.meta.env.VITE_VISUAL_PREVIEW !== '1') return;
    const requested = new URLSearchParams(window.location.search).get('preview');
    if (!requested) return;
    const state: PreviewState = requested === 'ready' ? 'ready' : requested === 'error' ? 'error' : 'setupRequired';
    const previewCard = (index: number) => `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 285 420"><defs><linearGradient id="g" x2="1" y2="1"><stop stop-color="#7661d8"/><stop offset="1" stop-color="#25334b"/></linearGradient></defs><rect width="285" height="420" rx="28" fill="url(#g)"/><circle cx="142" cy="175" r="80" fill="#7ce3c3" opacity=".32"/><text x="142" y="205" text-anchor="middle" fill="white" font-size="72" font-family="Arial" font-weight="700">${index + 1}</text><rect x="24" y="328" width="237" height="55" rx="14" fill="#101827" opacity=".8"/></svg>`)}`;
    const previewMatches = [0, 1].map((matchIndex) => ({
        battleTime: new Date(Date.now() - (matchIndex + 1) * 19 * 60_000).toISOString(),
        averageElixir: matchIndex === 0 ? 3.4 : 3.8,
        cards: Array.from({length: 8}, (_, cardIndex) => ({
            id: 26000000 + cardIndex,
            name: `Card ${cardIndex + 1}`,
            imageUrl: previewCard(cardIndex),
            form: cardIndex === 0 ? 'evolution' : 'normal',
            isSpell: cardIndex > 5,
        })),
    }));

    const previewApp = {
        GetStatus: async () => ({state, hasToken: state !== 'setupRequired', errorCode: state === 'error' ? 'apiConnection' : ''}),
        GetAppInfo: async () => ({
            version: '0.3.3-preview',
            dataDirectory: 'C:\\Users\\Preview\\AppData\\Local\\RoyaleScout',
            tokenStorage: 'windows-dpapi',
        }),
        ConfigureAPIToken: async () => ({success: false, code: 'invalid'}),
        DeleteAllLocalData: async () => ({success: true, code: 'ok'}),
        Search: async () => ({candidates: [{playerTag: '#PREVIEW', nickname: 'Preview Player', trophies: 2910, clanName: 'Royale Scout'}]}),
        GetDecks: async () => ({playerTag: '#PREVIEW', matches: previewMatches}),
        GetTopPlayers: async () => ({
            updatedAt: new Date(Date.now() - 2 * 60_000).toISOString(),
            players: Array.from({length: 250}, (_, index) => ({
                playerTag: `#PREVIEW${index}`,
                nickname: ['Mohamed Light', 'Mugi', 'Ryley', 'Ian77', 'Lucas'][index % 5] + (index > 4 ? ` ${index + 1}` : ''),
                rank: index + 1,
                rankedRating: 3268 - Math.floor(index * 1.7),
                clanTag: index % 7 === 0 ? '' : '#CLAN',
                clanName: index % 7 === 0 ? '' : ['SK Gaming', 'FUT Esports', 'Tribe Gaming'][index % 3],
                clanBadgeId: 0,
            })),
        }),
        RefreshTopPlayers: async () => previewApp.GetTopPlayers(),
        GetTopDecks: async (tags: string[]) => ({players: tags.map((playerTag) => ({playerTag, matches: previewMatches.slice(0, 1), errorCode: ''}))}),
        CancelTopDeckLoading: async () => ({success: true, code: 'ok'}),
    };

    const previewWindow = window as typeof window & {
        go?: {main: {App: typeof previewApp}};
        runtime?: {BrowserOpenURL?: (url: string) => void};
    };
    previewWindow.go = {main: {App: previewApp}};
    previewWindow.runtime = previewWindow.runtime ?? {};
    previewWindow.runtime.BrowserOpenURL = () => undefined;
}
