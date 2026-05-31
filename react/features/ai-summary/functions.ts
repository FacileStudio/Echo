import { IReduxState } from '../app/types';

import { STORE_NAME } from './constants';

export function isAISummaryEnabled(state: IReduxState): boolean {
    return state['features/base/config']?.ai?.enabled ?? false;
}

export function getAISummaryState(state: IReduxState) {
    return state[STORE_NAME];
}

export function isAISummaryPanelOpen(state: IReduxState): boolean {
    return state[STORE_NAME]?.isPanelOpen ?? false;
}

export function getAISummaryStatus(state: IReduxState) {
    return state[STORE_NAME]?.status ?? 'idle';
}

export function getAISummaryResult(state: IReduxState) {
    return state[STORE_NAME]?.result ?? null;
}

export function getTranscriptEntries(state: IReduxState) {
    return state[STORE_NAME]?.entries ?? [];
}

export function getAISummaryProxyUrl(state: IReduxState): string | undefined {
    return state['features/base/config']?.ai?.proxyUrl;
}

export function getAISummaryProvider(state: IReduxState): string {
    return state['features/base/config']?.ai?.provider ?? 'claude';
}

export function getAISummaryLanguage(state: IReduxState): string | undefined {
    return state['features/base/config']?.ai?.language;
}

export function shouldAutoSummarize(state: IReduxState): boolean {
    return state['features/base/config']?.ai?.autoSummarize ?? false;
}
