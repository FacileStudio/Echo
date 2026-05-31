import React, { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector, useStore } from 'react-redux';

import { IStore } from '../../../app/types';
import { toggleAISummaryPanel } from '../../actions';
import {
    getAISummaryProxyUrl,
    getAISummaryResult,
    getAISummaryStatus,
    getTranscriptEntries,
    isAISummaryPanelOpen
} from '../../functions';
import { generateSummary } from '../../summarizer';
import { ISummaryResult, ITranscriptEntry } from '../../types';

const AISummaryPanel = () => {
    const { t } = useTranslation();
    const dispatch = useDispatch();
    const store = useStore() as unknown as IStore;
    const isPanelOpen = useSelector(isAISummaryPanelOpen);
    const status = useSelector(getAISummaryStatus);
    const result = useSelector(getAISummaryResult);
    const entries = useSelector(getTranscriptEntries);

    const proxyUrl = useSelector(getAISummaryProxyUrl);

    const handleGenerate = useCallback(() => {
        generateSummary(store);
    }, [ store ]);

    const handleDownloadTranscript = useCallback(() => {
        if (entries.length === 0) {
            return;
        }
        const text = formatTranscriptAsMarkdown(entries);
        const blob = new Blob([ text ], { type: 'text/markdown' });
        const createURL = (URL as any).createObjectURL as (blob: Blob) => string;
        const revokeURL = (URL as any).revokeObjectURL as (url: string) => void;
        const url = createURL(blob);
        const a = document.createElement('a');

        a.href = url;
        a.download = `transcript-${new Date().toISOString().slice(0, 10)}.md`;
        a.click();
        revokeURL(url);
    }, [ entries ]);

    const handleCopy = useCallback(() => {
        if (result) {
            const text = formatSummaryAsMarkdown(result);

            navigator.clipboard.writeText(text);
        }
    }, [ result ]);

    const handleClose = useCallback(() => {
        dispatch(toggleAISummaryPanel());
    }, [ dispatch ]);

    if (!isPanelOpen) {
        return null;
    }

    return (
        <div className = 'ai-summary-panel'>
            <div className = 'ai-summary-panel-header'>
                <h3>{t('aiSummary.title')}</h3>
                <button
                    className = 'ai-summary-close-btn'
                    onClick = { handleClose }>
                    &times;
                </button>
            </div>

            <div className = 'ai-summary-panel-body'>
                {status === 'collecting' && (
                    <div className = 'ai-summary-collecting'>
                        <div className = 'ai-summary-collecting-header'>
                            <p className = 'ai-summary-count'>
                                {entries.length} {t('aiSummary.entriesCollected')}
                            </p>
                            {entries.length > 0 && (
                                <div className = 'ai-summary-actions'>
                                    {proxyUrl && (
                                        <button
                                            className = 'ai-summary-generate-btn'
                                            onClick = { handleGenerate }>
                                            {t('aiSummary.generateNow')}
                                        </button>
                                    )}
                                    <button
                                        className = 'ai-summary-download-btn'
                                        onClick = { handleDownloadTranscript }>
                                        {t('aiSummary.downloadTranscript')}
                                    </button>
                                </div>
                            )}
                        </div>
                        {entries.length > 0 && (
                            <div className = 'ai-summary-live-transcript'>
                                {entries.slice(-50).map((entry, i) => (
                                    <div
                                        className = 'ai-summary-live-entry'
                                        key = { i }>
                                        <span className = 'ai-summary-live-speaker'>
                                            {entry.speaker}
                                        </span>
                                        <span className = 'ai-summary-live-text'>
                                            {entry.text}
                                        </span>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                )}

                {status === 'generating' && (
                    <div className = 'ai-summary-status'>
                        <div className = 'ai-summary-spinner' />
                        <p>{t('aiSummary.generating')}</p>
                    </div>
                )}

                {status === 'error' && (
                    <div className = 'ai-summary-error'>
                        <p>{t('aiSummary.error')}</p>
                        <button
                            className = 'ai-summary-generate-btn'
                            onClick = { handleGenerate }>
                            {t('aiSummary.retry')}
                        </button>
                    </div>
                )}

                {status === 'ready' && result && (
                    <>
                        <SummaryDisplay
                            onCopy = { handleCopy }
                            result = { result } />
                        {entries.length > 0 && (
                            <div className = 'ai-summary-actions'>
                                <button
                                    className = 'ai-summary-download-btn'
                                    onClick = { handleDownloadTranscript }>
                                    {t('aiSummary.downloadTranscript')}
                                </button>
                            </div>
                        )}
                    </>
                )}

                {status === 'idle' && (
                    <div className = 'ai-summary-status'>
                        <p>{t('aiSummary.idle')}</p>
                        {entries.length > 0 && (
                            <button
                                className = 'ai-summary-download-btn'
                                onClick = { handleDownloadTranscript }>
                                {t('aiSummary.downloadTranscript')}
                            </button>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
};

interface ISummaryDisplayProps {
    onCopy: () => void;
    result: ISummaryResult;
}

const SummaryDisplay = ({ result, onCopy }: ISummaryDisplayProps) => {
    const { t } = useTranslation();

    return (
        <div className = 'ai-summary-result'>
            <section className = 'ai-summary-section'>
                <h4>{t('aiSummary.summary')}</h4>
                <p>{result.summary}</p>
            </section>

            {result.topics.length > 0 && (
                <section className = 'ai-summary-section'>
                    <h4>{t('aiSummary.topics')}</h4>
                    <ul>
                        {result.topics.map((topic, i) => (
                            <li key = { i }>{topic}</li>
                        ))}
                    </ul>
                </section>
            )}

            {result.decisions.length > 0 && (
                <section className = 'ai-summary-section'>
                    <h4>{t('aiSummary.decisions')}</h4>
                    <ul>
                        {result.decisions.map((decision, i) => (
                            <li key = { i }>{decision}</li>
                        ))}
                    </ul>
                </section>
            )}

            {result.actionItems.length > 0 && (
                <section className = 'ai-summary-section'>
                    <h4>{t('aiSummary.actionItems')}</h4>
                    <ul>
                        {result.actionItems.map((item, i) => (
                            <li key = { i }>
                                <span className = 'ai-summary-action-desc'>{item.description}</span>
                                {item.assignee && (
                                    <span className = 'ai-summary-action-assignee'>
                                        {` → ${item.assignee}`}
                                    </span>
                                )}
                                {item.deadline && (
                                    <span className = 'ai-summary-action-deadline'>
                                        {` (${item.deadline})`}
                                    </span>
                                )}
                            </li>
                        ))}
                    </ul>
                </section>
            )}

            {Object.keys(result.perSpeaker).length > 0 && (
                <section className = 'ai-summary-section'>
                    <h4>{t('aiSummary.perSpeaker')}</h4>
                    <ul>
                        {Object.entries(result.perSpeaker).map(([ speaker, contribution ]) => (
                            <li key = { speaker }>
                                <strong>{speaker}:</strong> {contribution}
                            </li>
                        ))}
                    </ul>
                </section>
            )}

            <div className = 'ai-summary-actions'>
                <button
                    className = 'ai-summary-copy-btn'
                    onClick = { onCopy }>
                    {t('aiSummary.copy')}
                </button>
            </div>
        </div>
    );
};

function formatSummaryAsMarkdown(result: ISummaryResult): string {
    let md = `# Meeting Summary\n\n${result.summary}\n`;

    if (result.topics.length > 0) {
        md += `\n## Key Topics\n${result.topics.map(t => `- ${t}`).join('\n')}\n`;
    }

    if (result.decisions.length > 0) {
        md += `\n## Decisions\n${result.decisions.map(d => `- ${d}`).join('\n')}\n`;
    }

    if (result.actionItems.length > 0) {
        md += `\n## Action Items\n${result.actionItems.map(a => {
            let line = `- ${a.description}`;

            if (a.assignee) {
                line += ` (${a.assignee})`;
            }
            if (a.deadline) {
                line += ` [${a.deadline}]`;
            }

            return line;
        }).join('\n')}\n`;
    }

    if (Object.keys(result.perSpeaker).length > 0) {
        md += `\n## Per-Speaker Contributions\n${
            Object.entries(result.perSpeaker)
                .map(([ s, c ]) => `- **${s}**: ${c}`)
                .join('\n')}\n`;
    }

    return md;
}

function formatTranscriptAsMarkdown(entries: ITranscriptEntry[]): string {
    const date = new Date().toISOString().slice(0, 10);
    let md = `# Meeting Transcript — ${date}\n\n`;

    for (const entry of entries) {
        const time = new Date(entry.timestamp).toLocaleTimeString();

        md += `**[${time}] ${entry.speaker}:** ${entry.text}\n\n`;
    }

    return md;
}

export default AISummaryPanel;
