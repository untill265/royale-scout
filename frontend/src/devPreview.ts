type PreviewState = 'setupRequired' | 'ready' | 'error';

export function installDevPreview() {
    if (!import.meta.env.DEV) return;
    const requested = new URLSearchParams(window.location.search).get('preview');
    if (!requested) return;
    const state: PreviewState = requested === 'ready' ? 'ready' : requested === 'error' ? 'error' : 'setupRequired';

    const previewApp = {
        GetStatus: async () => ({state, hasToken: state !== 'setupRequired', errorCode: state === 'error' ? 'apiConnection' : ''}),
        GetAppInfo: async () => ({
            version: '0.2.2-preview',
            dataDirectory: 'C:\\Users\\Preview\\AppData\\Local\\RoyaleScout',
            tokenStorage: 'windows-dpapi',
        }),
        ConfigureAPIToken: async () => ({success: false, code: 'invalid'}),
        DeleteAllLocalData: async () => ({success: true, code: 'ok'}),
        Search: async () => ({candidates: []}),
        GetDecks: async () => ({playerTag: '', matches: []}),
    };

    const previewWindow = window as typeof window & {
        go?: {main: {App: typeof previewApp}};
        runtime?: {BrowserOpenURL?: (url: string) => void};
    };
    previewWindow.go = {main: {App: previewApp}};
    previewWindow.runtime = previewWindow.runtime ?? {};
    previewWindow.runtime.BrowserOpenURL = () => undefined;
}
