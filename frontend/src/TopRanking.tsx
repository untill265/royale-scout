import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {CancelTopDeckLoading, GetTopDecks, GetTopPlayers, RefreshTopPlayers} from '../wailsjs/go/main/App';

type Locale = 'ru' | 'en';

interface TopPlayer {
    playerTag: string;
    nickname: string;
    rank: number;
    rankedRating: number;
    clanTag: string;
    clanName: string;
    clanBadgeId: number;
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
    averageElixir: number;
}

interface TopSnapshot {
    players: TopPlayer[];
    updatedAt: string;
}

interface TopPlayerDecks {
    playerTag: string;
    matches: DeckMatch[];
    errorCode: string;
}

interface TopDeckBatch {
    players: TopPlayerDecks[];
}

type DeckLoadState = {
    status: 'loading' | 'ready' | 'error';
    matches: DeckMatch[];
};

const copy = {
    ru: {
        eyebrow: 'Глобальный Ranked-рейтинг',
        title: 'Топ 250 игроков',
        waiting: 'Получаем свежий глобальный рейтинг…',
        waitingText: 'Это происходит в фоне и не замедляет поиск соперника.',
        refresh: 'Обновить список',
        refreshing: 'Обновляем…',
        searchPlaceholder: 'Найти игрока или клан',
        player: 'Игрок',
        rating: 'Медали',
        decks: 'Последняя Ranked-колода',
        medals: 'медалей',
        noClan: 'Без клана',
        noMatches: 'Нет недавних Ranked-матчей',
        unavailable: 'Battle log временно недоступен',
        loadingDecks: 'Подгружаем колоды…',
        retry: 'Повторить',
        noResults: 'В топе нет такого игрока или клана',
        deck: 'Колода',
        elixir: 'Средняя стоимость эликсира',
        justNow: 'только что',
        yesterday: 'вчера',
        unknownTime: 'время неизвестно',
        requestFailed: 'Не удалось получить топ. Проверьте подключение и попробуйте снова.',
        shown: 'Показано',
        players: 'игроков',
    },
    en: {
        eyebrow: 'Global Ranked leaderboard',
        title: 'Top 250 players',
        waiting: 'Loading the current global leaderboard…',
        waitingText: 'This happens in the background and does not slow down opponent search.',
        refresh: 'Refresh list',
        refreshing: 'Refreshing…',
        searchPlaceholder: 'Find a player or clan',
        player: 'Player',
        rating: 'Medals',
        decks: 'Latest Ranked deck',
        medals: 'medals',
        noClan: 'No clan',
        noMatches: 'No recent Ranked matches',
        unavailable: 'Battle log is temporarily unavailable',
        loadingDecks: 'Loading decks…',
        retry: 'Retry',
        noResults: 'No player or clan in the top matches this search',
        deck: 'Deck',
        elixir: 'Average elixir cost',
        justNow: 'just now',
        yesterday: 'yesterday',
        unknownTime: 'time unavailable',
        requestFailed: 'Could not load the ranking. Check your connection and try again.',
        shown: 'Showing',
        players: 'players',
    },
} as const;

export function TopRanking({locale}: {locale: Locale}) {
    const t = copy[locale];
    const [snapshot, setSnapshot] = useState<TopSnapshot>({players: [], updatedAt: ''});
    const [filter, setFilter] = useState('');
    const [initialLoading, setInitialLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);
    const [snapshotError, setSnapshotError] = useState(false);
    const [deckStates, setDeckStates] = useState<Record<string, DeckLoadState>>({});
    const [deckQueue, setDeckQueue] = useState<string[]>([]);
    const [batchLoading, setBatchLoading] = useState(false);
    const mountedRef = useRef(true);
    const requestedRef = useRef(new Set<string>());

    const loadSnapshot = useCallback(async (manual = false) => {
        if (manual) setRefreshing(true);
        try {
            const response = await (manual ? RefreshTopPlayers() : GetTopPlayers()) as TopSnapshot;
            if (!mountedRef.current) return;
            setSnapshot({players: response.players ?? [], updatedAt: response.updatedAt ?? ''});
            setSnapshotError(false);
        } catch {
            if (mountedRef.current) setSnapshotError(true);
        } finally {
            if (mountedRef.current) {
                setInitialLoading(false);
                setRefreshing(false);
            }
        }
    }, []);

    useEffect(() => {
        mountedRef.current = true;
        void loadSnapshot();
        const timer = window.setInterval(() => void loadSnapshot(), 30_000);
        return () => {
            mountedRef.current = false;
            window.clearInterval(timer);
            void CancelTopDeckLoading().catch(() => undefined);
        };
    }, [loadSnapshot]);

    const queuePlayer = useCallback((playerTag: string) => {
        if (requestedRef.current.has(playerTag)) return;
        requestedRef.current.add(playerTag);
        setDeckQueue((queued) => [...queued, playerTag]);
    }, []);

    const retryPlayer = useCallback((playerTag: string) => {
        requestedRef.current.delete(playerTag);
        setDeckStates((current) => {
            const next = {...current};
            delete next[playerTag];
            return next;
        });
        queuePlayer(playerTag);
    }, [queuePlayer]);

    useEffect(() => {
        if (batchLoading || deckQueue.length === 0) return;
        const batch = deckQueue.slice(0, 8);
        setDeckQueue((queued) => queued.slice(batch.length));
        setBatchLoading(true);
        setDeckStates((current) => {
            const next = {...current};
            for (const playerTag of batch) next[playerTag] = {status: 'loading', matches: []};
            return next;
        });

        void GetTopDecks(batch).then((response) => {
            if (!mountedRef.current) return;
            const result = response as TopDeckBatch;
            const byTag = new Map((result.players ?? []).map((player) => [player.playerTag, player]));
            setDeckStates((current) => {
                const next = {...current};
                for (const playerTag of batch) {
                    const loaded = byTag.get(playerTag);
                    next[playerTag] = loaded && !loaded.errorCode
                        ? {status: 'ready', matches: loaded.matches ?? []}
                        : {status: 'error', matches: []};
                }
                return next;
            });
        }).catch(() => {
            if (!mountedRef.current) return;
            setDeckStates((current) => {
                const next = {...current};
                for (const playerTag of batch) next[playerTag] = {status: 'error', matches: []};
                return next;
            });
        }).finally(() => {
            if (mountedRef.current) setBatchLoading(false);
        });
    }, [batchLoading, deckQueue]);

    const visiblePlayers = useMemo(() => {
        const query = filter.trim().toLocaleLowerCase(locale);
        if (!query) return snapshot.players;
        return snapshot.players.filter((player) =>
            player.nickname.toLocaleLowerCase(locale).includes(query) ||
            player.clanName.toLocaleLowerCase(locale).includes(query) ||
            player.clanTag.toLocaleLowerCase(locale).includes(query),
        );
    }, [filter, locale, snapshot.players]);

    if (initialLoading || (!snapshotError && snapshot.players.length === 0)) {
        return (
            <section className="top-ranking-section">
                <TopHero locale={locale} refreshing={false} onRefresh={() => void loadSnapshot(true)}/>
                <div className="top-waiting-state">
                    <span className="top-waiting-orbit"><RankingIcon/><i/><i/></span>
                    <h2>{t.waiting}</h2>
                    <p>{t.waitingText}</p>
                    <div className="top-progress"><span/></div>
                </div>
            </section>
        );
    }

    return (
        <section className="top-ranking-section">
            <TopHero locale={locale} refreshing={refreshing} onRefresh={() => void loadSnapshot(true)}/>

            <div className="top-toolbar">
                <label className="top-filter">
                    <SearchIcon/>
                    <input
                        value={filter}
                        onChange={(event) => setFilter(event.target.value)}
                        placeholder={t.searchPlaceholder}
                        autoComplete="off"
                        spellCheck={false}
                    />
                    {filter && <button type="button" onClick={() => setFilter('')} aria-label={t.searchPlaceholder}>×</button>}
                </label>
                <span className="top-result-count">{t.shown} <strong>{visiblePlayers.length}</strong> {t.players}</span>
            </div>

            {snapshotError && <div className="top-inline-error"><AlertIcon/>{t.requestFailed}</div>}

            <div className="top-list-head" aria-hidden="true">
                <span>#</span><span>{t.player}</span><span>{t.rating}</span><span>{t.decks}</span>
            </div>

            {visiblePlayers.length > 0 ? (
                <div className="top-player-list">
                    {visiblePlayers.map((player) => (
                        <TopPlayerRow
                            key={player.playerTag}
                            player={player}
                            decks={deckStates[player.playerTag]}
                            locale={locale}
                            onVisible={queuePlayer}
                            onRetry={retryPlayer}
                        />
                    ))}
                </div>
            ) : (
                <div className="top-empty-filter"><SearchIcon/><p>{t.noResults}</p></div>
            )}
        </section>
    );
}

function TopHero({
    locale,
    refreshing,
    onRefresh,
}: {
    locale: Locale;
    refreshing: boolean;
    onRefresh: () => void;
}) {
    const t = copy[locale];
    return (
        <div className="top-ranking-hero">
            <div>
                <p className="eyebrow"><span/>{t.eyebrow}</p>
                <h1>{t.title}</h1>
            </div>
            <div className="top-live-card">
                <button type="button" onClick={onRefresh} disabled={refreshing}>
                    <RefreshIcon className={refreshing ? 'rotating' : ''}/>{refreshing ? t.refreshing : t.refresh}
                </button>
            </div>
        </div>
    );
}

function TopPlayerRow({
    player,
    decks,
    locale,
    onVisible,
    onRetry,
}: {
    player: TopPlayer;
    decks?: DeckLoadState;
    locale: Locale;
    onVisible: (playerTag: string) => void;
    onRetry: (playerTag: string) => void;
}) {
    const t = copy[locale];
    const rowRef = useRef<HTMLElement>(null);
    const [nearViewport, setNearViewport] = useState(false);

    useEffect(() => {
        const row = rowRef.current;
        if (!row || typeof IntersectionObserver === 'undefined') {
            setNearViewport(true);
            onVisible(player.playerTag);
            return;
        }
        const observer = new IntersectionObserver((entries) => {
            const visible = entries.some((entry) => entry.isIntersecting);
            setNearViewport(visible);
            if (visible) onVisible(player.playerTag);
        }, {rootMargin: '650px 0px'});
        observer.observe(row);
        return () => observer.disconnect();
    }, [onVisible, player.playerTag]);

    const podium = player.rank <= 3 ? ` podium-${player.rank}` : '';
    return (
        <article ref={rowRef} className={`top-player-row${podium}`}>
            <span className="top-rank"><small>TOP</small>{player.rank}</span>
            <div className="top-player-identity">
                <ClanBadge badgeId={player.clanBadgeId} clanName={player.clanName}/>
                <div>
                    <strong title={player.nickname}>{player.nickname}</strong>
                    <span title={player.clanName || t.noClan}>{player.clanName || t.noClan}</span>
                </div>
            </div>
            <div className="top-rating"><MedalIcon/><strong>{player.rankedRating}</strong><span>{t.medals}</span></div>
            <div className="top-decks-area">
                {!nearViewport ? (
                    <DeckPlaceholder loaded={decks?.status === 'ready'}/>
                ) : !decks || decks.status === 'loading' ? (
                    <DeckSkeleton label={t.loadingDecks}/>
                ) : decks.status === 'error' ? (
                    <div className="top-deck-message error"><AlertIcon/><span>{t.unavailable}</span><button type="button" onClick={() => onRetry(player.playerTag)}>{t.retry}</button></div>
                ) : decks.matches.length === 0 ? (
                    <div className="top-deck-message"><CardsIcon/><span>{t.noMatches}</span></div>
                ) : (
                    <div className="top-deck-stack">
                        {decks.matches.slice(0, 1).map((match, index) => (
                            <CompactDeck key={`${match.battleTime}-${index}`} match={match} index={index} locale={locale} renderImages={nearViewport}/>
                        ))}
                    </div>
                )}
            </div>
        </article>
    );
}

function CompactDeck({match, index, locale, renderImages}: {match: DeckMatch; index: number; locale: Locale; renderImages: boolean}) {
    const t = copy[locale];
    return (
        <div className="top-deck-strip">
            <div className="top-deck-time" title={formatExactTime(match.battleTime, locale)}>
                <strong>{t.deck} {index + 1}</strong>
                <span><ClockIcon/>{formatRelativeTime(match.battleTime, locale)}</span>
            </div>
            <div className="top-card-row">
                {match.cards.map((card, cardIndex) => (
                    <span className="top-card" key={`${card.id}-${cardIndex}`} title={card.name}>
                        {renderImages && <img src={card.imageUrl} alt={card.name} loading="lazy" draggable={false}/>}
                    </span>
                ))}
            </div>
            <ElixirCost value={match.averageElixir} label={t.elixir}/>
        </div>
    );
}

function DeckSkeleton({label}: {label: string}) {
    return (
        <div className="top-deck-skeleton" aria-label={label}>
            {[0].map((row) => (
                <div key={row}><span className="skeleton-label"/>{Array.from({length: 8}, (_, index) => <i key={index}/>)}</div>
            ))}
            <small>{label}</small>
        </div>
    );
}

function DeckPlaceholder({loaded}: {loaded: boolean}) {
    return (
        <div className={`top-deck-placeholder${loaded ? ' loaded' : ''}`} aria-hidden="true">
            <span/>
        </div>
    );
}

function ClanBadge({badgeId, clanName}: {badgeId: number; clanName: string}) {
    const [failed, setFailed] = useState(false);
    if (!badgeId || failed) return <span className="clan-badge fallback"><ClanIcon/></span>;
    return (
        <span className="clan-badge">
            <img src={`/clan-badges/${badgeId}.png`} alt={clanName} loading="lazy" draggable={false} onError={() => setFailed(true)}/>
        </span>
    );
}

function ElixirCost({value, label}: {value: number; label: string}) {
    return <span className="elixir-cost" title={label}><ElixirIcon/><strong>{value > 0 ? value.toFixed(1) : '—'}</strong></span>;
}

function formatRelativeTime(value: string, locale: Locale) {
    if (!value) return copy[locale].unknownTime;
    const playedAt = new Date(value).getTime();
    if (Number.isNaN(playedAt)) return copy[locale].unknownTime;
    const seconds = Math.max(0, Math.round((Date.now() - playedAt) / 1000));
    if (seconds < 45) return copy[locale].justNow;
    const formatter = new Intl.RelativeTimeFormat(locale, {numeric: 'always'});
    const minutes = Math.round(seconds / 60);
    if (minutes < 60) return formatter.format(-minutes, 'minute');
    const hours = Math.round(minutes / 60);
    if (hours < 24) return formatter.format(-hours, 'hour');
    const days = Math.round(hours / 24);
    if (days === 1) return copy[locale].yesterday;
    return formatter.format(-days, 'day');
}

function formatExactTime(value: string, locale: Locale) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return copy[locale].unknownTime;
    return new Intl.DateTimeFormat(locale, {dateStyle: 'medium', timeStyle: 'short'}).format(date);
}

const SearchIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/></svg>;
const MedalIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3h8l-1.5 5h-5z"/><circle cx="12" cy="15" r="5"/><path d="M12 12.5v5m-2.5-2.5h5"/></svg>;
const RankingIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 20V10h4v10m2 0V4h4v16m2 0v-7h4v7"/><path d="M3 20h19"/></svg>;
const CardsIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="3" width="14" height="18" rx="2"/><path d="m9 13 3-3 3 3-3 3z"/></svg>;
const AlertIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 2.8 20h18.4z"/><path d="M12 9v4m0 3v.1"/></svg>;
const ClockIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="8.5"/><path d="M12 7v5l3 2"/></svg>;
const ClanIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5.5c0 4.2 2.7 7.2 7 8.8 4.3-1.6 7-4.6 7-8.8V6z"/><path d="m9 12 2 2 4-4"/></svg>;
const RefreshIcon = ({className = ''}: {className?: string}) => <svg className={className} viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4v6h6"/><path d="M5.5 15a7 7 0 1 0 .5-7"/></svg>;
export const ElixirIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2.7c-2.8 4.1-6.2 7.9-6.2 12.1A6.2 6.2 0 0 0 12 21a6.2 6.2 0 0 0 6.2-6.2C18.2 10.6 14.8 6.8 12 2.7Z"/><path d="M9.1 14.2c.2 1.7 1.2 2.7 2.8 3"/></svg>;
