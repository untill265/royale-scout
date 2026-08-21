import {FormEvent, ReactNode, useEffect, useMemo, useRef, useState} from 'react';
import {
    ConfigureAPIToken,
    DeleteAllLocalData,
    GetAppInfo,
    GetDecks,
    GetStatus,
    Search as searchPlayers,
} from '../wailsjs/go/main/App';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import './App.css';

type Locale = 'ru' | 'en';
type Screen = 'search' | 'candidates' | 'decks';
type AppState = 'setupRequired' | 'initializing' | 'ready' | 'waitingForSeason' | 'error';

interface StatusResponse {
    state: AppState;
    hasToken: boolean;
    errorCode: string;
}

interface AppInfoResponse {
    version: string;
    dataDirectory: string;
    tokenStorage: string;
}

interface ActionResponse {
    success: boolean;
    code: string;
}

interface Candidate {
    playerTag: string;
    nicknameOriginal: string;
    rankedRating: number;
    countryCode: string;
}

interface SearchResponse {
    candidates: Candidate[];
}

interface DeckCard {
    id: number;
    name: string;
    imageUrl: string;
    form: 'normal' | 'evolution' | 'hero';
    isSpell: boolean;
}

interface DeckMatch {
    battleTime: string;
    cards: DeckCard[];
}

interface DeckResponse {
    playerTag: string;
    matches: DeckMatch[];
}

const API_PORTAL_URL = 'https://developer.clashroyale.com/';
const FAN_CONTENT_POLICY_URL = 'https://supercell.com/en/fan-content-policy/';

const translations = {
    ru: {
        appName: 'Royale Scout',
        moduleName: 'Opponent Deck Scout',
        eyebrow: 'Быстрый разведчик колод',
        title: 'Найдите колоду соперника\nза несколько секунд',
        description: 'Введите Ranked-медали и желательно часть ника — так поиск точнее. Мы покажем до трёх последних Ranked-колод.',
        inputLabel: 'Медали и часть ника',
        inputPlaceholder: 'Например: 2910 су',
        scriptLabel: 'Есть иероглифы',
        scriptHint: 'Китайский, японский, корейский или арабский текст',
        scriptNote: 'Эмоции не считаются',
        helpButton: 'Как искать',
        helpTooltip: 'Открыть подсказку по поиску',
        helpEyebrow: 'Всё просто',
        helpTitle: 'Найдём соперника за пару секунд',
        helpIntro: 'Посмотрите на экран соперника в игре: введите его медали и желательно часть ника — так поиск будет точнее и сможет сразу выдать нужного игрока. Тег игрока не нужен.',
        helpExampleFragment: 'су',
        helpExampleMedals: 'медали',
        helpExampleNickname: 'часть ника · желательно',
        helpSteps: [
            {
                title: 'Начните с медалей',
                text: 'Впишите число медалей соперника. Затем через пробел желательно добавить часть ника — так меньше шанс попасть в длинный список совпадений. Поиск только по медалям тоже работает.',
                example: null,
            },
            {
                title: 'Ник целиком не нужен',
                text: 'Хватит нескольких букв. Регистр и украшения ника можно не повторять, а русский и латинский варианты приложение попробует сопоставить само.',
                example: 'Суслик = suslik',
            },
            {
                title: 'Точное число не требуется',
                text: 'Поиск автоматически проверит диапазон ±48 от введённого числа, поэтому медали не обязательно указывать идеально точно.',
                example: '2910 → 2862–2958',
            },
            {
                title: 'Как работает «Есть иероглифы»',
                text: 'Это дополнительный фильтр. Без флажка поиск проверяет все ники — в том числе с китайскими, японскими, корейскими или арабскими символами. Включите его, чтобы оставить только такие ники. Эмоции не считаются.',
                example: null,
            },
            {
                title: 'Что произойдёт после поиска',
                text: 'Если подходит один игрок, его колоды откроются сразу. Если совпадений несколько, появится список — выберите в нём нужного соперника. Затем вы увидите до трёх последних Ranked-колод, а заклинания будут продублированы ниже.',
                example: null,
            },
        ],
        helpFooter: 'Всё готово? Нажмите Enter или «Найти соперника». Новый поиск начинается кнопкой «Сбросить / Новый поиск».',
        helpClose: 'Понятно',
        helpCloseLabel: 'Закрыть инструкцию',
        search: 'Найти соперника',
        searching: 'Ищем…',
        enterHint: 'Enter — начать поиск',
        rangeHint: 'Поиск автоматически охватывает ±48 медалей',
        initializingTitle: 'Подготавливаем базу игроков',
        initializingText: 'Первый запуск занимает немного времени. После подготовки поле активируется автоматически.',
        waitingTitle: 'Начинается новый Ranked-сезон',
        waitingText: 'Ждём появления нового глобального списка игроков.',
        connectionTitle: 'Не удалось подключиться к Clash Royale API',
        connectionText: 'Проверьте интернет, API-ключ и разрешённый IP. Приложение попробует снова автоматически.',
        invalidInput: 'Начните запрос с количества Ranked-медалей.',
        notFound: 'Игрок не найден.',
        requestFailed: 'Не удалось выполнить запрос. Попробуйте ещё раз.',
        candidatesEyebrow: 'Найдено несколько игроков',
        candidatesTitle: 'Выберите соперника',
        candidatesText: 'Откройте нужный оригинальный ник из списка.',
        medal: 'медалей',
        newSearch: 'Сбросить / Новый поиск',
        backToResults: 'Назад к результатам',
        decksEyebrow: 'Последние Ranked-матчи',
        decksTitle: 'Колоды игрока',
        noData: 'Нет данных',
        noDataText: 'В доступном battle log нет Ranked-матчей.',
        match: 'Матч',
        unknownTime: 'Время неизвестно',
        justNow: 'только что',
        yesterday: 'вчера',
        freshness: 'Battle log может появляться с небольшой задержкой.',
        loadingDecks: 'Загружаем колоды…',
        spells: 'Заклинания',
        noSpells: 'В этой колоде нет заклинаний',
        language: 'Язык',
        settingsButton: 'Настройки',
        setupEyebrow: 'Безопасный первый запуск',
        setupTitle: 'Подключите официальный Clash Royale API',
        setupText: 'Игровой аккаунт и тег не нужны. Создайте личный ключ на портале разработчиков Supercell — приложение проверит его и сохранит только на этом компьютере.',
        setupSteps: [
            {title: 'Откройте API-портал', text: 'Войдите или создайте отдельную учётную запись на developer.clashroyale.com.'},
            {title: 'Создайте ключ', text: 'Откройте My Account → Create New Key и укажите свой текущий публичный IPv4 в списке разрешённых адресов.'},
            {title: 'Вставьте токен ниже', text: 'Скопируйте токен целиком. Мы сразу проверим его через официальный API и сохраним в защищённом хранилище Windows.'},
        ],
        openPortal: 'Открыть официальный API-портал',
        tokenLabel: 'Личный API-токен',
        tokenPlaceholder: 'Вставьте токен полностью',
        showToken: 'Показать токен',
        hideToken: 'Скрыть токен',
        saveToken: 'Проверить и сохранить',
        verifyingToken: 'Проверяем токен…',
        tokenPrivacy: 'Токен шифруется для вашей учётной записи Windows и используется только для запросов к api.clashroyale.com.',
        ipTip: 'Если интернет-провайдер сменит ваш публичный IP, обновите адрес в ключе на портале или создайте новый ключ, затем замените токен в настройках.',
        tokenErrors: {
            empty: 'Вставьте токен целиком.',
            invalid: 'Supercell отклонил токен. Проверьте, что он скопирован полностью и ещё активен.',
            ipNotAllowed: 'Этот IP не разрешён для ключа. Укажите текущий публичный IPv4 на портале Supercell.',
            rateLimited: 'Слишком много запросов. Подождите немного и попробуйте снова.',
            serviceUnavailable: 'API Supercell временно недоступен. Попробуйте чуть позже.',
            network: 'Не удалось связаться с API. Проверьте интернет и повторите попытку.',
            verificationFailed: 'Не получилось проверить токен. Попробуйте ещё раз.',
            localStorage: 'Не удалось сохранить токен на компьютере.',
            internal: 'Токен принят, но приложение не смогло запуститься. Перезапустите его.',
        },
        settingsTitle: 'Настройки и приватность',
        settingsSubtitle: 'Всё важное о ключе, данных и соединениях — в одном месте.',
        settingsClose: 'Закрыть настройки',
        apiSection: 'Официальный API',
        apiActive: 'Токен подключён и защищён',
        apiMissing: 'Токен ещё не подключён',
        apiProtected: 'Сохранённый токен нельзя посмотреть: его можно только заменить новым.',
        changeToken: 'Заменить токен',
        cancelChange: 'Отмена',
        tokenSaved: 'Новый токен проверен и сохранён.',
        privacySection: 'Что остаётся личным',
        privacyLead: 'У Royale Scout нет аккаунтов, аналитики, истории поисков и собственного сервера.',
        privacyPoints: [
            'Введённые ники и медали не сохраняются.',
            'Токен и рабочая база лежат только на этом компьютере.',
            'Токен и игровые запросы идут только в официальный Clash Royale API; изображения карт — с официального CDN Supercell.',
        ],
        storageSection: 'Локальные данные',
        storageText: 'Здесь находятся защищённый токен, база рейтинга и кэш изображений. Удаление вернёт приложение к первому запуску.',
        dataFolder: 'Папка данных',
        deleteData: 'Удалить все локальные данные',
        deleteWarning: 'Будут удалены токен, база и кэш. Это действие нельзя отменить.',
        confirmDelete: 'Да, удалить всё',
        cancelDelete: 'Оставить данные',
        deleting: 'Удаляем…',
        deleteFailed: 'Не удалось удалить локальные данные. Закройте приложение и попробуйте ещё раз.',
        version: 'Версия',
        legalSection: 'Важно',
        legalText: 'Приложение использует официальный API, не меняет игру и не управляет ей. Перед использованием на турнире уточните правила конкретного организатора.',
        legalDisclaimer: "This material is unofficial and is not endorsed by Supercell. For more information see Supercell's Fan Content Policy.",
        policyLink: 'Fan Content Policy',
        footerPrivacy: 'Приватность',
        footerUnofficial: 'Неофициальный проект, не одобренный Supercell',
        statusOpenSettings: 'Открыть настройки',
    },
    en: {
        appName: 'Royale Scout',
        moduleName: 'Opponent Deck Scout',
        eyebrow: 'Fast deck intelligence',
        title: 'Find your opponent’s deck\nin a few seconds',
        description: 'Enter Ranked medals and preferably part of the nickname for a more precise match. See up to three recent Ranked decks.',
        inputLabel: 'Medals and nickname fragment',
        inputPlaceholder: 'For example: 2910 su',
        scriptLabel: 'Contains hieroglyphs',
        scriptHint: 'Chinese, Japanese, Korean, or Arabic text',
        scriptNote: 'Emoji do not count',
        helpButton: 'How to search',
        helpTooltip: 'Open the search tips',
        helpEyebrow: 'It’s easy',
        helpTitle: 'Find your opponent in seconds',
        helpIntro: 'Check the opponent screen in-game: enter their medal count and preferably part of the nickname. This makes the search more precise and can take you straight to the right player. No player tag needed.',
        helpExampleFragment: 'su',
        helpExampleMedals: 'medals',
        helpExampleNickname: 'nickname fragment · recommended',
        helpSteps: [
            {
                title: 'Start with the medals',
                text: 'Type your opponent’s medal count. Then add part of the nickname after a space if possible—this helps avoid a long list of matches. Searching by medals alone still works.',
                example: null,
            },
            {
                title: 'You do not need the full nickname',
                text: 'A few letters are enough. Do not worry about uppercase letters or decorations—the app will also try to match Cyrillic and Latin spellings.',
                example: 'Суслик = suslik',
            },
            {
                title: 'The medal count can be close',
                text: 'Search automatically covers ±48 from the number you entered, so the medal count does not have to be perfectly exact.',
                example: '2910 → 2862–2958',
            },
            {
                title: 'How “Contains hieroglyphs” works',
                text: 'This is an extra filter. With it off, every nickname is searched—including ones with Chinese, Japanese, Korean, or Arabic characters. Turn it on to keep only those nicknames. Emoji do not count.',
                example: null,
            },
            {
                title: 'What happens after searching',
                text: 'If one player matches, their decks open immediately. If several match, you will get a list and can choose the right opponent. Then you will see up to three recent Ranked decks, with spells repeated below.',
                example: null,
            },
        ],
        helpFooter: 'Ready? Press Enter or “Find opponent”. Start over with “Reset / New search”.',
        helpClose: 'Got it',
        helpCloseLabel: 'Close search guide',
        search: 'Find opponent',
        searching: 'Searching…',
        enterHint: 'Enter — start search',
        rangeHint: 'Search automatically covers ±48 medals',
        initializingTitle: 'Preparing the player database',
        initializingText: 'The first launch takes a moment. The field will activate automatically when ready.',
        waitingTitle: 'A new Ranked season is starting',
        waitingText: 'Waiting for the new global player list to appear.',
        connectionTitle: 'Could not connect to the Clash Royale API',
        connectionText: 'Check your connection, API key, and allowed IP. The app will retry automatically.',
        invalidInput: 'Start the search with the Ranked medal count.',
        notFound: 'Player not found.',
        requestFailed: 'The request failed. Please try again.',
        candidatesEyebrow: 'Multiple players found',
        candidatesTitle: 'Choose your opponent',
        candidatesText: 'Select the correct original nickname from the list.',
        medal: 'medals',
        newSearch: 'Reset / New search',
        backToResults: 'Back to results',
        decksEyebrow: 'Recent Ranked matches',
        decksTitle: 'Player decks',
        noData: 'No data',
        noDataText: 'There are no Ranked matches in the available battle log.',
        match: 'Match',
        unknownTime: 'Time unavailable',
        justNow: 'just now',
        yesterday: 'yesterday',
        freshness: 'The battle log may appear with a short delay.',
        loadingDecks: 'Loading decks…',
        spells: 'Spells',
        noSpells: 'This deck has no spells',
        language: 'Language',
        settingsButton: 'Settings',
        setupEyebrow: 'Secure first launch',
        setupTitle: 'Connect the official Clash Royale API',
        setupText: 'No game login or player tag is needed. Create your own key in Supercell’s developer portal—the app verifies it and stores it only on this PC.',
        setupSteps: [
            {title: 'Open the API portal', text: 'Sign in or create a separate account at developer.clashroyale.com.'},
            {title: 'Create a key', text: 'Go to My Account → Create New Key and add your current public IPv4 to the allowed addresses.'},
            {title: 'Paste the token below', text: 'Copy the complete token. We will verify it with the official API and save it in Windows protected storage.'},
        ],
        openPortal: 'Open the official API portal',
        tokenLabel: 'Personal API token',
        tokenPlaceholder: 'Paste the complete token',
        showToken: 'Show token',
        hideToken: 'Hide token',
        saveToken: 'Verify and save',
        verifyingToken: 'Verifying token…',
        tokenPrivacy: 'Your token is encrypted for your Windows account and is used only for requests to api.clashroyale.com.',
        ipTip: 'If your provider changes your public IP, update the key on the portal or create a new key, then replace the token in Settings.',
        tokenErrors: {
            empty: 'Paste the complete token.',
            invalid: 'Supercell rejected this token. Check that it was copied completely and is still active.',
            ipNotAllowed: 'This IP is not allowed for the key. Add your current public IPv4 in the Supercell portal.',
            rateLimited: 'Too many requests. Wait a moment and try again.',
            serviceUnavailable: 'The Supercell API is temporarily unavailable. Try again later.',
            network: 'Could not reach the API. Check your connection and try again.',
            verificationFailed: 'The token could not be verified. Please try again.',
            localStorage: 'The token could not be saved on this PC.',
            internal: 'The token was accepted, but the app could not start. Restart the app.',
        },
        settingsTitle: 'Settings & privacy',
        settingsSubtitle: 'Your key, local data, and network use in one clear place.',
        settingsClose: 'Close settings',
        apiSection: 'Official API',
        apiActive: 'Token connected and protected',
        apiMissing: 'No token connected yet',
        apiProtected: 'The saved token is never shown again. It can only be replaced with a newly verified token.',
        changeToken: 'Replace token',
        cancelChange: 'Cancel',
        tokenSaved: 'The new token has been verified and saved.',
        privacySection: 'What stays private',
        privacyLead: 'Royale Scout has no accounts, analytics, search history, or server of its own.',
        privacyPoints: [
            'Entered nicknames and medal counts are not stored.',
            'The token and working database remain on this PC.',
            'The token and game requests go only to the official Clash Royale API; card images come from Supercell’s official CDN.',
        ],
        storageSection: 'Local data',
        storageText: 'This folder contains the protected token, ranking database, and image cache. Deleting it returns the app to first launch.',
        dataFolder: 'Data folder',
        deleteData: 'Delete all local data',
        deleteWarning: 'Your token, database, and cache will be deleted. This cannot be undone.',
        confirmDelete: 'Yes, delete everything',
        cancelDelete: 'Keep my data',
        deleting: 'Deleting…',
        deleteFailed: 'Local data could not be deleted. Close the app and try again.',
        version: 'Version',
        legalSection: 'Important',
        legalText: 'The app uses the official API and does not modify or control the game. Check the specific organiser’s rules before using it during a tournament.',
        legalDisclaimer: "This material is unofficial and is not endorsed by Supercell. For more information see Supercell's Fan Content Policy.",
        policyLink: 'Fan Content Policy',
        footerPrivacy: 'Privacy',
        footerUnofficial: 'Unofficial project, not endorsed by Supercell',
        statusOpenSettings: 'Open settings',
    },
} as const;

function App() {
    const [locale, setLocale] = useState<Locale>(() => (localStorage.getItem('royale-scout-locale') === 'en' ? 'en' : 'ru'));
    const [status, setStatus] = useState<AppState>('initializing');
    const [hasToken, setHasToken] = useState(false);
    const [appInfo, setAppInfo] = useState<AppInfoResponse | null>(null);
    const [screen, setScreen] = useState<Screen>('search');
    const [input, setInput] = useState('');
    const [requireTargetScript, setRequireTargetScript] = useState(false);
    const [candidates, setCandidates] = useState<Candidate[]>([]);
    const [selectedPlayer, setSelectedPlayer] = useState<Candidate | null>(null);
    const [decks, setDecks] = useState<DeckResponse | null>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [helpOpen, setHelpOpen] = useState(false);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [replacingToken, setReplacingToken] = useState(false);
    const [token, setToken] = useState('');
    const [tokenVisible, setTokenVisible] = useState(false);
    const [tokenBusy, setTokenBusy] = useState(false);
    const [tokenMessage, setTokenMessage] = useState<string | null>(null);
    const [tokenSaved, setTokenSaved] = useState(false);
    const [deleteConfirm, setDeleteConfirm] = useState(false);
    const [deleteBusy, setDeleteBusy] = useState(false);
    const [deleteError, setDeleteError] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const helpTriggerRef = useRef<HTMLButtonElement>(null);
    const helpCloseRef = useRef<HTMLButtonElement>(null);
    const settingsCloseRef = useRef<HTMLButtonElement>(null);
    const t = translations[locale];

    useEffect(() => {
        document.documentElement.lang = locale;
        localStorage.setItem('royale-scout-locale', locale);
    }, [locale]);

    useEffect(() => {
        let active = true;
        const updateStatus = async () => {
            try {
                const response = await GetStatus() as StatusResponse;
                if (active) {
                    setStatus(response.state);
                    setHasToken(response.hasToken);
                }
            } catch {
                if (active) setStatus('error');
            }
        };
        void updateStatus();
        const timer = window.setInterval(updateStatus, 1000);
        return () => {
            active = false;
            window.clearInterval(timer);
        };
    }, []);

    useEffect(() => {
        let active = true;
        void GetAppInfo().then((response) => {
            if (active) setAppInfo(response as AppInfoResponse);
        }).catch(() => undefined);
        return () => { active = false; };
    }, []);

    useEffect(() => {
        if (status === 'ready' && screen === 'search' && !busy) {
            window.setTimeout(() => inputRef.current?.focus(), 40);
        }
    }, [status, screen, busy]);

    useEffect(() => {
        if (!helpOpen) return;
        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = 'hidden';
        window.setTimeout(() => helpCloseRef.current?.focus(), 40);
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setHelpOpen(false);
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => {
            document.body.style.overflow = previousOverflow;
            window.removeEventListener('keydown', handleKeyDown);
            window.setTimeout(() => helpTriggerRef.current?.focus(), 40);
        };
    }, [helpOpen]);

    useEffect(() => {
        if (!settingsOpen) return;
        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = 'hidden';
        window.setTimeout(() => settingsCloseRef.current?.focus(), 40);
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape' && !deleteBusy && !tokenBusy) closeSettings();
        };
        window.addEventListener('keydown', handleKeyDown);
        return () => {
            document.body.style.overflow = previousOverflow;
            window.removeEventListener('keydown', handleKeyDown);
        };
    }, [settingsOpen, deleteBusy, tokenBusy]);

    const titleLines = useMemo(() => t.title.split('\n'), [t.title]);

    async function handleSearch(event: FormEvent) {
        event.preventDefault();
        if (busy || status !== 'ready') return;
        if (!/^\s*\d+/.test(input)) {
            setError(t.invalidInput);
            inputRef.current?.focus();
            return;
        }

        setBusy(true);
        setError(null);
        try {
            const response = await searchPlayers(input, requireTargetScript) as SearchResponse;
            const found = response.candidates ?? [];
            if (found.length === 0) {
                setError(t.notFound);
                inputRef.current?.focus();
                return;
            }
            setCandidates(found);
            if (found.length === 1) {
                await openPlayer(found[0]);
            } else {
                setScreen('candidates');
            }
        } catch {
            setError(t.requestFailed);
        } finally {
            setBusy(false);
        }
    }

    async function openPlayer(candidate: Candidate) {
        setBusy(true);
        setError(null);
        setSelectedPlayer(candidate);
        try {
            const response = await GetDecks(candidate.playerTag) as DeckResponse;
            setDecks(response);
            setScreen('decks');
        } catch {
            setError(t.requestFailed);
        } finally {
            setBusy(false);
        }
    }

    function resetSearch() {
        setInput('');
        setRequireTargetScript(false);
        setCandidates([]);
        setSelectedPlayer(null);
        setDecks(null);
        setError(null);
        setScreen('search');
        window.setTimeout(() => inputRef.current?.focus(), 40);
    }

    function backToCandidates() {
        setDecks(null);
        setSelectedPlayer(null);
        setError(null);
        setScreen('candidates');
    }

    function switchLocale(nextLocale: Locale) {
        setLocale(nextLocale);
        setError(null);
    }

    function resetTokenForm() {
        setToken('');
        setTokenVisible(false);
        setTokenMessage(null);
        setTokenSaved(false);
    }

    function closeSettings() {
        if (tokenBusy || deleteBusy) return;
        setSettingsOpen(false);
        setReplacingToken(false);
        setDeleteConfirm(false);
        setDeleteError(false);
        resetTokenForm();
    }

    async function saveAPIToken(event: FormEvent, fromSettings: boolean) {
        event.preventDefault();
        if (tokenBusy) return;
        setTokenBusy(true);
        setTokenMessage(null);
        setTokenSaved(false);
        try {
            const result = await ConfigureAPIToken(token) as ActionResponse;
            if (!result.success) {
                setTokenMessage(tokenErrorText(result.code, locale));
                return;
            }
            setToken('');
            setTokenVisible(false);
            setHasToken(true);
            setStatus('initializing');
            if (fromSettings) {
                setReplacingToken(false);
                setTokenSaved(true);
            }
        } catch {
            setTokenMessage(tokenErrorText('verificationFailed', locale));
        } finally {
            setTokenBusy(false);
        }
    }

    async function deleteLocalData() {
        if (deleteBusy) return;
        setDeleteBusy(true);
        setDeleteError(false);
        try {
            const result = await DeleteAllLocalData() as ActionResponse;
            if (!result.success) {
                setDeleteError(true);
                return;
            }
            resetSearch();
            setHasToken(false);
            setStatus('setupRequired');
            closeSettingsAfterDelete();
        } catch {
            setDeleteError(true);
        } finally {
            setDeleteBusy(false);
        }
    }

    function closeSettingsAfterDelete() {
        setSettingsOpen(false);
        setReplacingToken(false);
        setDeleteConfirm(false);
        resetTokenForm();
    }

    return (
        <div className="app-shell">
            <div className="ambient ambient-one"/>
            <div className="ambient ambient-two"/>

            <header className="topbar">
                <button className="brand" type="button" onClick={resetSearch} aria-label={t.appName}>
                    <span className="brand-mark" aria-hidden="true"><span>R</span></span>
                    <span className="brand-copy">
                        <strong>{t.appName}</strong>
                        <small>{t.moduleName}</small>
                    </span>
                </button>

                <div className="topbar-actions">
                    <button
                        className="settings-trigger"
                        type="button"
                        onClick={() => setSettingsOpen(true)}
                        aria-label={t.settingsButton}
                        title={t.settingsButton}
                    ><SettingsIcon/><span>{t.settingsButton}</span></button>
                    <div className="language-switch" role="group" aria-label={t.language}>
                        <button className={locale === 'ru' ? 'active' : ''} type="button" onClick={() => switchLocale('ru')}>RU</button>
                        <button className={locale === 'en' ? 'active' : ''} type="button" onClick={() => switchLocale('en')}>EN</button>
                    </div>
                </div>
            </header>

            <main>
                {status === 'setupRequired' ? (
                    <SetupView
                        locale={locale}
                        token={token}
                        setToken={(value) => {
                            setToken(value);
                            setTokenMessage(null);
                        }}
                        tokenVisible={tokenVisible}
                        setTokenVisible={setTokenVisible}
                        busy={tokenBusy}
                        message={tokenMessage}
                        onSubmit={(event) => void saveAPIToken(event, false)}
                    />
                ) : status !== 'ready' ? (
                    <StatusView state={status} locale={locale} onOpenSettings={() => setSettingsOpen(true)}/>
                ) : screen === 'search' ? (
                    <section className="search-layout">
                        <div className="hero-copy">
                            <p className="eyebrow"><span/>{t.eyebrow}</p>
                            <h1>{titleLines.map((line, index) => <span key={line}>{line}{index === 0 && <br/>}</span>)}</h1>
                            <p className="hero-description">{t.description}</p>

                            <div className="range-note">
                                <RangeIcon/>
                                <span>{t.rangeHint}</span>
                            </div>
                        </div>

                        <form className="search-card" onSubmit={handleSearch}>
                            <div className="search-card-accent"/>
                            <div className="search-card-header">
                                <label htmlFor="opponent-query">{t.inputLabel}</label>
                                <button
                                    ref={helpTriggerRef}
                                    className="help-trigger"
                                    type="button"
                                    onClick={() => setHelpOpen(true)}
                                    aria-haspopup="dialog"
                                    title={t.helpTooltip}
                                >
                                    <InfoIcon/>
                                    <span>{t.helpButton}</span>
                                    <span className="help-trigger-tooltip" aria-hidden="true">{t.helpTooltip}</span>
                                </button>
                            </div>
                            <div className={`query-control ${error ? 'has-error' : ''}`}>
                                <MedalIcon/>
                                <input
                                    ref={inputRef}
                                    id="opponent-query"
                                    value={input}
                                    onChange={(event) => {
                                        setInput(event.target.value);
                                        if (error) setError(null);
                                    }}
                                    placeholder={t.inputPlaceholder}
                                    autoFocus
                                    autoComplete="off"
                                    spellCheck={false}
                                    maxLength={256}
                                    disabled={busy}
                                />
                                {input && !busy && (
                                    <button className="clear-input" type="button" onClick={() => {
                                        setInput('');
                                        setError(null);
                                        inputRef.current?.focus();
                                    }} aria-label={t.newSearch}>×</button>
                                )}
                            </div>

                            <label className="script-filter">
                                <input
                                    type="checkbox"
                                    checked={requireTargetScript}
                                    onChange={(event) => setRequireTargetScript(event.target.checked)}
                                    disabled={busy}
                                />
                                <span className="custom-check"><CheckIcon/></span>
                                <span className="filter-copy">
                                    <strong>{t.scriptLabel}</strong>
                                    <small>{t.scriptHint}</small>
                                    <small className="filter-note">{t.scriptNote}</small>
                                </span>
                            </label>

                            {error && <div className="inline-error" role="alert"><AlertIcon/>{error}</div>}

                            <button className="primary-button" type="submit" disabled={busy || input.trim() === ''}>
                                {busy ? <Spinner/> : <SearchIcon/>}
                                <span>{busy ? t.searching : t.search}</span>
                                {!busy && <kbd>↵</kbd>}
                            </button>
                            <p className="shortcut-hint">{t.enterHint}</p>
                        </form>
                    </section>
                ) : screen === 'candidates' ? (
                    <section className="results-section">
                        <div className="results-heading-row">
                            <SectionHeading eyebrow={t.candidatesEyebrow} title={t.candidatesTitle} text={t.candidatesText}/>
                            <button className="secondary-button" type="button" onClick={resetSearch}><ResetIcon/>{t.newSearch}</button>
                        </div>
                        {error && <div className="page-error" role="alert"><AlertIcon/>{error}</div>}
                        <div className="candidate-list">
                            {candidates.map((candidate, index) => (
                                <button
                                    key={candidate.playerTag}
                                    className="candidate-row"
                                    type="button"
                                    onClick={() => void openPlayer(candidate)}
                                    disabled={busy}
                                >
                                    <span className="candidate-index">{String(index + 1).padStart(2, '0')}</span>
                                    <span className="candidate-name">{candidate.nicknameOriginal}</span>
                                    <span className="candidate-meta">
                                        <span>{candidate.rankedRating} {t.medal}</span>
                                        {candidate.countryCode && <span className="country-code">{candidate.countryCode}</span>}
                                    </span>
                                    <ChevronIcon/>
                                </button>
                            ))}
                        </div>
                        {busy && <div className="loading-overlay"><Spinner/><span>{t.loadingDecks}</span></div>}
                    </section>
                ) : (
                    <section className="decks-section">
                        <div className="deck-heading-row">
                            <SectionHeading eyebrow={t.decksEyebrow} title={selectedPlayer?.nicknameOriginal ?? t.decksTitle}/>
                            <div className="deck-actions">
                                {candidates.length > 1 && (
                                    <button className="ghost-button" type="button" onClick={backToCandidates}><BackIcon/>{t.backToResults}</button>
                                )}
                                <button className="secondary-button" type="button" onClick={resetSearch}><ResetIcon/>{t.newSearch}</button>
                            </div>
                        </div>

                        {decks && decks.matches.length > 0 ? (
                            <div className="deck-list">
                                {decks.matches.map((match, index) => {
                                    const spells = match.cards.filter((card) => card.isSpell);
                                    return (
                                    <article className="deck-panel" key={`${match.battleTime}-${index}`}>
                                        <div className="match-heading">
                                            <span className="match-number">{String(index + 1).padStart(2, '0')}</span>
                                            <div>
                                                <strong>{t.match} {index + 1}</strong>
                                                <small>{formatRelativeTime(match.battleTime, locale, t)}</small>
                                            </div>
                                        </div>
                                        <div className="card-grid">
                                            {match.cards.map((card, cardIndex) => (
                                                <div className="deck-card" key={`${card.id}-${cardIndex}`} title={card.name}>
                                                    <img src={card.imageUrl} alt={card.name} draggable={false}/>
                                                </div>
                                            ))}
                                        </div>

                                        <div className="spells-summary">
                                            <div className="spells-heading">
                                                <span className="spells-icon"><SpellIcon/></span>
                                                <strong>{t.spells}</strong>
                                                <span className="spells-count">{spells.length}</span>
                                            </div>
                                            {spells.length > 0 ? (
                                                <div className="spell-list">
                                                    {spells.map((card, spellIndex) => (
                                                        <div className="spell-card" key={`${card.id}-spell-${spellIndex}`} title={card.name}>
                                                            <img src={card.imageUrl} alt="" draggable={false}/>
                                                            <span className="spell-name">{card.name}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            ) : <p className="no-spells">{t.noSpells}</p>}
                                        </div>
                                    </article>
                                )})}
                            </div>
                        ) : (
                            <div className="empty-state">
                                <span className="empty-icon"><CardsIcon/></span>
                                <h2>{t.noData}</h2>
                                <p>{t.noDataText}</p>
                            </div>
                        )}
                        <p className="freshness-note"><InfoIcon/>{t.freshness}</p>
                    </section>
                )}
            </main>

            {helpOpen && (
                <div
                    className="help-backdrop"
                    onMouseDown={(event) => {
                        if (event.target === event.currentTarget) setHelpOpen(false);
                    }}
                >
                    <section
                        className="help-dialog"
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="search-help-title"
                    >
                        <button
                            ref={helpCloseRef}
                            className="help-dialog-close"
                            type="button"
                            onClick={() => setHelpOpen(false)}
                            aria-label={t.helpCloseLabel}
                        >×</button>

                        <p className="eyebrow"><span/>{t.helpEyebrow}</p>
                        <h2 id="search-help-title">{t.helpTitle}</h2>
                        <p className="help-intro">{t.helpIntro}</p>

                        <div className="help-query-example" aria-label={t.inputPlaceholder}>
                            <div className="help-query-value">
                                <span>2910</span>
                                <span>{t.helpExampleFragment}</span>
                            </div>
                            <div className="help-query-labels">
                                <span>{t.helpExampleMedals}</span>
                                <span>{t.helpExampleNickname}</span>
                            </div>
                        </div>

                        <div className="help-steps">
                            {t.helpSteps.map((step, index) => (
                                <article className="help-step" key={step.title}>
                                    <span className="help-step-number">{String(index + 1).padStart(2, '0')}</span>
                                    <div>
                                        <h3>{step.title}</h3>
                                        <p>{step.text}</p>
                                        {step.example && <code>{step.example}</code>}
                                    </div>
                                </article>
                            ))}
                        </div>

                        <div className="help-dialog-footer">
                            <p><InfoIcon/>{t.helpFooter}</p>
                            <button className="primary-button help-confirm" type="button" onClick={() => setHelpOpen(false)}>{t.helpClose}</button>
                        </div>
                    </section>
                </div>
            )}

            {settingsOpen && (
                <div
                    className="help-backdrop"
                    onMouseDown={(event) => {
                        if (event.target === event.currentTarget && !tokenBusy && !deleteBusy) closeSettings();
                    }}
                >
                    <section className="help-dialog settings-dialog" role="dialog" aria-modal="true" aria-labelledby="settings-title">
                        <button
                            ref={settingsCloseRef}
                            className="help-dialog-close"
                            type="button"
                            onClick={closeSettings}
                            disabled={tokenBusy || deleteBusy}
                            aria-label={t.settingsClose}
                        >×</button>

                        <p className="eyebrow"><span/>{t.settingsButton}</p>
                        <h2 id="settings-title">{t.settingsTitle}</h2>
                        <p className="help-intro">{t.settingsSubtitle}</p>

                        <div className="settings-sections">
                            <section className="settings-section">
                                <div className="settings-section-heading">
                                    <span className="settings-section-icon"><KeyIcon/></span>
                                    <div><h3>{t.apiSection}</h3><p>{hasToken ? t.apiActive : t.apiMissing}</p></div>
                                    <span className={`status-pill ${hasToken ? 'active' : ''}`}>{hasToken ? 'OK' : '—'}</span>
                                </div>
                                <p className="settings-note">{t.apiProtected}</p>
                                {tokenSaved && <div className="success-message" role="status"><CheckIcon/>{t.tokenSaved}</div>}
                                {replacingToken ? (
                                    <TokenForm
                                        locale={locale}
                                        token={token}
                                        setToken={(value) => {
                                            setToken(value);
                                            setTokenMessage(null);
                                            setTokenSaved(false);
                                        }}
                                        tokenVisible={tokenVisible}
                                        setTokenVisible={setTokenVisible}
                                        busy={tokenBusy}
                                        message={tokenMessage}
                                        onSubmit={(event) => void saveAPIToken(event, true)}
                                        compact
                                    >
                                        <button className="ghost-button" type="button" disabled={tokenBusy} onClick={() => {
                                            setReplacingToken(false);
                                            resetTokenForm();
                                        }}>{t.cancelChange}</button>
                                    </TokenForm>
                                ) : (
                                    <button className="secondary-button compact-button" type="button" onClick={() => {
                                        setReplacingToken(true);
                                        resetTokenForm();
                                    }}><RefreshKeyIcon/>{t.changeToken}</button>
                                )}
                            </section>

                            <section className="settings-section">
                                <div className="settings-section-heading">
                                    <span className="settings-section-icon"><ShieldIcon/></span>
                                    <div><h3>{t.privacySection}</h3><p>{t.privacyLead}</p></div>
                                </div>
                                <ul className="privacy-list">
                                    {t.privacyPoints.map((point) => <li key={point}><CheckIcon/><span>{point}</span></li>)}
                                </ul>
                            </section>

                            <section className="settings-section">
                                <div className="settings-section-heading">
                                    <span className="settings-section-icon"><StorageIcon/></span>
                                    <div><h3>{t.storageSection}</h3><p>{t.storageText}</p></div>
                                </div>
                                {appInfo?.dataDirectory && (
                                    <div className="data-path"><span>{t.dataFolder}</span><code>{appInfo.dataDirectory}</code></div>
                                )}
                                {!deleteConfirm ? (
                                    <button className="danger-button" type="button" onClick={() => {
                                        setDeleteConfirm(true);
                                        setDeleteError(false);
                                    }}>{t.deleteData}</button>
                                ) : (
                                    <div className="delete-confirm">
                                        <p><AlertIcon/>{t.deleteWarning}</p>
                                        {deleteError && <div className="inline-error" role="alert"><AlertIcon/>{t.deleteFailed}</div>}
                                        <div>
                                            <button className="danger-button solid" type="button" disabled={deleteBusy} onClick={() => void deleteLocalData()}>
                                                {deleteBusy ? <Spinner/> : <DeleteIcon/>}{deleteBusy ? t.deleting : t.confirmDelete}
                                            </button>
                                            <button className="ghost-button" type="button" disabled={deleteBusy} onClick={() => setDeleteConfirm(false)}>{t.cancelDelete}</button>
                                        </div>
                                    </div>
                                )}
                            </section>

                            <section className="settings-section legal-section">
                                <div className="settings-section-heading">
                                    <span className="settings-section-icon"><InfoIcon/></span>
                                    <div><h3>{t.legalSection}</h3><p>{t.legalText}</p></div>
                                </div>
                                <p className="legal-disclaimer">{t.legalDisclaimer}</p>
                                <button className="text-link" type="button" onClick={() => BrowserOpenURL(FAN_CONTENT_POLICY_URL)}>{t.policyLink}<ExternalIcon/></button>
                            </section>
                        </div>

                        <div className="settings-meta">
                            <span>Royale Scout</span><span>·</span><span>{t.version} {appInfo?.version ?? '0.2.1'}</span>
                        </div>
                    </section>
                </div>
            )}

            <footer>
                <span>Royale Scout</span><span className="footer-dot"/>
                <span>{t.footerUnofficial}</span>
                <button type="button" onClick={() => setSettingsOpen(true)}>{t.footerPrivacy}</button>
                <button type="button" onClick={() => BrowserOpenURL(FAN_CONTENT_POLICY_URL)}>{t.policyLink}<ExternalIcon/></button>
            </footer>
        </div>
    );
}

function SetupView({
    locale,
    token,
    setToken,
    tokenVisible,
    setTokenVisible,
    busy,
    message,
    onSubmit,
}: {
    locale: Locale;
    token: string;
    setToken: (value: string) => void;
    tokenVisible: boolean;
    setTokenVisible: (value: boolean) => void;
    busy: boolean;
    message: string | null;
    onSubmit: (event: FormEvent) => void;
}) {
    const t = translations[locale];
    return (
        <section className="setup-layout">
            <div className="setup-copy">
                <p className="eyebrow"><span/>{t.setupEyebrow}</p>
                <h1>{t.setupTitle}</h1>
                <p className="hero-description">{t.setupText}</p>
                <button className="portal-button" type="button" onClick={() => BrowserOpenURL(API_PORTAL_URL)}>
                    <ExternalIcon/>{t.openPortal}
                </button>
                <div className="setup-steps">
                    {t.setupSteps.map((step, index) => (
                        <article key={step.title}>
                            <span>{String(index + 1).padStart(2, '0')}</span>
                            <div><h2>{step.title}</h2><p>{step.text}</p></div>
                        </article>
                    ))}
                </div>
            </div>
            <div className="setup-card">
                <div className="setup-card-accent"/>
                <span className="setup-shield"><ShieldIcon/></span>
                <h2>{t.tokenLabel}</h2>
                <p>{t.tokenPrivacy}</p>
                <TokenForm
                    locale={locale}
                    token={token}
                    setToken={setToken}
                    tokenVisible={tokenVisible}
                    setTokenVisible={setTokenVisible}
                    busy={busy}
                    message={message}
                    onSubmit={onSubmit}
                />
                <div className="ip-tip"><InfoIcon/><span>{t.ipTip}</span></div>
                <p className="setup-disclaimer">{t.legalDisclaimer}</p>
            </div>
        </section>
    );
}

function TokenForm({
    locale,
    token,
    setToken,
    tokenVisible,
    setTokenVisible,
    busy,
    message,
    onSubmit,
    compact = false,
    children,
}: {
    locale: Locale;
    token: string;
    setToken: (value: string) => void;
    tokenVisible: boolean;
    setTokenVisible: (value: boolean) => void;
    busy: boolean;
    message: string | null;
    onSubmit: (event: FormEvent) => void;
    compact?: boolean;
    children?: ReactNode;
}) {
    const t = translations[locale];
    return (
        <form className={`token-form ${compact ? 'compact' : ''}`} onSubmit={onSubmit}>
            <label htmlFor={compact ? 'replacement-token' : 'setup-token'}>{t.tokenLabel}</label>
            <div className={`token-control ${message ? 'has-error' : ''}`}>
                <KeyIcon/>
                <input
                    id={compact ? 'replacement-token' : 'setup-token'}
                    type={tokenVisible ? 'text' : 'password'}
                    value={token}
                    onChange={(event) => setToken(event.target.value)}
                    placeholder={t.tokenPlaceholder}
                    autoComplete="off"
                    spellCheck={false}
                    maxLength={8192}
                    disabled={busy}
                />
                <button type="button" onClick={() => setTokenVisible(!tokenVisible)} disabled={busy} aria-label={tokenVisible ? t.hideToken : t.showToken}>
                    {tokenVisible ? <EyeOffIcon/> : <EyeIcon/>}
                </button>
            </div>
            {message && <div className="inline-error" role="alert"><AlertIcon/>{message}</div>}
            <div className="token-form-actions">
                <button className="primary-button" type="submit" disabled={busy || token.trim() === ''}>
                    {busy ? <Spinner/> : <ShieldIcon/>}<span>{busy ? t.verifyingToken : t.saveToken}</span>
                </button>
                {children}
            </div>
        </form>
    );
}

function StatusView({state, locale, onOpenSettings}: {state: AppState; locale: Locale; onOpenSettings: () => void}) {
    const t = translations[locale];
    const isWaiting = state === 'waitingForSeason';
    const isError = state === 'error';
    return (
        <section className="status-view" aria-live="polite">
            <div className={`status-orbit ${isError ? 'error' : ''}`}>
                <span className="orbit-one"/><span className="orbit-two"/>
                {isError ? <AlertIcon/> : isWaiting ? <SeasonIcon/> : <CrownIcon/>}
            </div>
            <p className="eyebrow"><span/>{translations[locale].moduleName}</p>
            <h1>{isError ? t.connectionTitle : isWaiting ? t.waitingTitle : t.initializingTitle}</h1>
            <p>{isError ? t.connectionText : isWaiting ? t.waitingText : t.initializingText}</p>
            {!isError && <div className="progress-line"><span/></div>}
            {isError && <button className="secondary-button" type="button" onClick={onOpenSettings}><SettingsIcon/>{t.statusOpenSettings}</button>}
        </section>
    );
}

function tokenErrorText(code: string, locale: Locale) {
    const errors = translations[locale].tokenErrors as Record<string, string>;
    return errors[code] ?? errors.verificationFailed;
}

function SectionHeading({eyebrow, title, text}: {eyebrow: string; title: string; text?: string}) {
    return (
        <div className="section-heading">
            <p className="eyebrow"><span/>{eyebrow}</p>
            <h1>{title}</h1>
            {text && <p>{text}</p>}
        </div>
    );
}

function formatRelativeTime(value: string, locale: Locale, t: (typeof translations)[Locale]) {
    if (!value) return t.unknownTime;
    const playedAt = new Date(value).getTime();
    if (Number.isNaN(playedAt)) return t.unknownTime;
    const seconds = Math.max(0, Math.round((Date.now() - playedAt) / 1000));
    if (seconds < 45) return t.justNow;
    const formatter = new Intl.RelativeTimeFormat(locale, {numeric: 'always'});
    const minutes = Math.round(seconds / 60);
    if (minutes < 60) return formatter.format(-minutes, 'minute');
    const hours = Math.round(minutes / 60);
    if (hours < 24) return formatter.format(-hours, 'hour');
    const days = Math.round(hours / 24);
    if (days === 1) return t.yesterday;
    return formatter.format(-days, 'day');
}

const SearchIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/></svg>;
const MedalIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3h8l-1.5 5h-5z"/><circle cx="12" cy="15" r="5"/><path d="m12 12 1 2 2 .3-1.5 1.5.4 2.2-1.9-1-1.9 1 .4-2.2L9 14.3l2-.3z"/></svg>;
const RangeIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12h16M7 9l-3 3 3 3m10-6 3 3-3 3"/></svg>;
const CheckIcon = () => <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m4 10 4 4 8-9"/></svg>;
const AlertIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 2.8 20h18.4z"/><path d="M12 9v4m0 3v.1"/></svg>;
const ChevronIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg>;
const ResetIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4v6h6"/><path d="M5.5 15a7 7 0 1 0 .5-7"/></svg>;
const BackIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m14 5-7 7 7 7"/><path d="M7 12h13"/></svg>;
const InfoIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v.1"/></svg>;
const CardsIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="3" width="14" height="18" rx="2"/><path d="m9 13 3-3 3 3-3 3z"/></svg>;
const SpellIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m13 2-8 12h6l-1 8 9-13h-6z"/></svg>;
const CrownIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 7 5 4 4-7 4 7 5-4-2 11H5z"/><path d="M6 21h12"/></svg>;
const SeasonIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v18M3 12h18M5.6 5.6l12.8 12.8m0-12.8L5.6 18.4"/><circle cx="12" cy="12" r="3"/></svg>;
const SettingsIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1z"/></svg>;
const KeyIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="15" r="4"/><path d="m11 12 8-8m-3 3 3 3m-6 0 2 2"/></svg>;
const ShieldIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 4.5 6v5.5c0 4.6 3 7.9 7.5 9.5 4.5-1.6 7.5-4.9 7.5-9.5V6z"/><path d="m8.5 12 2.2 2.2 4.8-5"/></svg>;
const RefreshKeyIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="15" r="3"/><path d="m10 13 7-7m-2 0h3v3M5 6a7.5 7.5 0 0 1 12.5 2M19 18a7.5 7.5 0 0 1-12.5-2"/></svg>;
const StorageIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></svg>;
const DeleteIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6"/></svg>;
const EyeIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6z"/><circle cx="12" cy="12" r="2.5"/></svg>;
const EyeOffIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 3 18 18M10.5 6.1A9.7 9.7 0 0 1 12 6c6 0 9.5 6 9.5 6a15.5 15.5 0 0 1-2.2 2.9M6.4 7.4A16.4 16.4 0 0 0 2.5 12S6 18 12 18a9.5 9.5 0 0 0 3.1-.5M9.9 9.9a3 3 0 0 0 4.2 4.2"/></svg>;
const ExternalIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 5h5v5M13 11l6-6M19 13v6H5V5h6"/></svg>;
const Spinner = () => <span className="spinner" aria-hidden="true"/>;

export default App;
