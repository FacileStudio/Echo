import { IStore } from '../app/types';

import { setAISummaryError, setAISummaryResult, setAISummaryStatus } from './actions';
import {
    APPROX_TOKENS_PER_WORD,
    CHUNK_OVERLAP_ENTRIES,
    CHUNK_SIZE_TOKENS,
    MAX_SINGLE_PASS_TOKENS
} from './constants';
import { getAISummaryLanguage, getAISummaryProxyUrl, getTranscriptEntries } from './functions';
import logger from './logger';
import { IActionItem, ISummaryResult, ITranscriptEntry } from './types';

const SYSTEM_PROMPT = `You are a meeting analyst. Given a video conference transcript with speaker labels and timestamps, produce a structured JSON summary.

Output ONLY valid JSON matching this schema:
{
  "summary": "2-4 sentence executive summary of the meeting",
  "topics": ["key topic 1", "key topic 2"],
  "decisions": ["decision 1 with who decided", "decision 2"],
  "actionItems": [{"description": "task", "assignee": "name or null", "deadline": "date or null"}],
  "perSpeaker": {"Speaker Name": "1-sentence contribution summary"}
}

Rules:
- Be concise. Omit filler, greetings, and off-topic tangents.
- For action items, extract the assignee name if mentioned. Use null if unclear.
- For deadlines, use ISO date format if a specific date is mentioned, otherwise null.
- The summary should capture the most important outcomes.
- Topics should be distinct discussion themes, not individual statements.
- Decisions are explicit agreements or choices made during the meeting.`;

export async function generateSummary(store: IStore): Promise<void> {
    const state = store.getState();
    const entries = getTranscriptEntries(state);
    const proxyUrl = getAISummaryProxyUrl(state);
    const language = getAISummaryLanguage(state);

    if (!proxyUrl) {
        store.dispatch(setAISummaryError('AI summary proxy URL not configured'));

        return;
    }

    if (entries.length === 0) {
        store.dispatch(setAISummaryError('No transcript entries to summarize'));

        return;
    }

    store.dispatch(setAISummaryStatus('generating'));

    try {
        const transcript = formatTranscript(entries);
        const wordCount = transcript.split(/\s+/).length;
        const estimatedTokens = wordCount * APPROX_TOKENS_PER_WORD;

        let rawResult: string;

        if (estimatedTokens <= MAX_SINGLE_PASS_TOKENS) {
            rawResult = await callLLM(proxyUrl, buildPrompt(transcript, language, state));
        } else {
            rawResult = await mapReduceSummarize(proxyUrl, entries, language, state);
        }

        const parsed = parseResult(rawResult);

        store.dispatch(setAISummaryResult(parsed));
        logger.info('Summary generated successfully');
    } catch (error: any) {
        logger.error('Summary generation failed', error);
        store.dispatch(setAISummaryError(error?.message || 'Summary generation failed'));
    }
}

function formatTranscript(entries: ITranscriptEntry[]): string {
    return entries
        .map(e => `[${e.speaker}]: ${e.text}`)
        .join('\n');
}

function buildPrompt(transcript: string, language: string | undefined, state: any): string {
    const participants = state['features/ai-summary']?.participants ?? {};
    const participantList = Object.values(participants)
        .map((p: any) => `- ${p.displayName} (${p.role})`)
        .join('\n');

    let prompt = '';

    if (participantList) {
        prompt += `Participants:\n${participantList}\n\n`;
    }

    prompt += `Transcript:\n${transcript}`;

    if (language && language !== 'en') {
        prompt += `\n\nIMPORTANT: Generate the summary in ${language}.`;
    }

    return prompt;
}

async function mapReduceSummarize(
        proxyUrl: string,
        entries: ITranscriptEntry[],
        language: string | undefined,
        state: any
): Promise<string> {
    const chunks = chunkEntries(entries);
    const chunkSummaries: string[] = [];

    for (const chunk of chunks) {
        const transcript = formatTranscript(chunk);
        const prompt = `Summarize this meeting segment. Extract topics, decisions, and action items.\n\nTranscript:\n${transcript}`;
        const result = await callLLM(proxyUrl, prompt);

        chunkSummaries.push(result);
    }

    const mergePrompt = `You have ${chunkSummaries.length} segment summaries from a single meeting. Merge them into one cohesive summary.\n\nSegment summaries:\n${chunkSummaries.map((s, i) => `--- Segment ${i + 1} ---\n${s}`).join('\n\n')}\n\n${language && language !== 'en' ? `Generate the final summary in ${language}.` : ''}`;

    return callLLM(proxyUrl, mergePrompt);
}

function chunkEntries(entries: ITranscriptEntry[]): ITranscriptEntry[][] {
    const chunks: ITranscriptEntry[][] = [];
    let currentChunk: ITranscriptEntry[] = [];
    let currentTokens = 0;

    for (const entry of entries) {
        const entryTokens = entry.text.split(/\s+/).length * APPROX_TOKENS_PER_WORD;

        if (currentTokens + entryTokens > CHUNK_SIZE_TOKENS && currentChunk.length > 0) {
            chunks.push(currentChunk);
            currentChunk = currentChunk.slice(-CHUNK_OVERLAP_ENTRIES);
            currentTokens = currentChunk.reduce(
                (sum, e) => sum + (e.text.split(/\s+/).length * APPROX_TOKENS_PER_WORD), 0);
        }

        currentChunk.push(entry);
        currentTokens += entryTokens;
    }

    if (currentChunk.length > 0) {
        chunks.push(currentChunk);
    }

    return chunks;
}

async function callLLM(proxyUrl: string, userMessage: string): Promise<string> {
    return callWithRetry(async () => {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 90000);

        try {
            const response = await fetch(proxyUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    system: SYSTEM_PROMPT,
                    message: userMessage
                }),
                signal: controller.signal
            });

            if (!response.ok) {
                const error: any = new Error(`LLM proxy returned ${response.status}`);

                error.status = response.status;
                throw error;
            }

            const data = await response.json();

            return data.content || data.text || data.message || JSON.stringify(data);
        } finally {
            clearTimeout(timeout);
        }
    });
}

async function callWithRetry(fn: () => Promise<string>, maxRetries = 3, baseDelay = 1000): Promise<string> {
    for (let attempt = 0; attempt <= maxRetries; attempt++) {
        try {
            return await fn();
        } catch (error: any) {
            if (attempt === maxRetries || !isRetryable(error)) {
                throw error;
            }

            const delay = baseDelay * Math.pow(2, attempt) + (Math.random() * 1000);

            await new Promise(r => setTimeout(r, delay));
        }
    }

    throw new Error('Unreachable');
}

function isRetryable(error: any): boolean {
    const status = error?.status;

    return status === 429 || status === 500 || status === 503 || status === 529;
}

function parseResult(raw: string): ISummaryResult {
    let jsonStr = raw;
    const jsonMatch = raw.match(/```(?:json)?\s*([\s\S]*?)```/);

    if (jsonMatch) {
        jsonStr = jsonMatch[1].trim();
    }

    const parsed = JSON.parse(jsonStr);

    return {
        summary: parsed.summary || '',
        topics: parsed.topics || [],
        decisions: parsed.decisions || [],
        actionItems: (parsed.actionItems || []).map((item: any): IActionItem => ({
            description: item.description || item.task || '',
            assignee: item.assignee || null,
            deadline: item.deadline || null
        })),
        perSpeaker: parsed.perSpeaker || {},
        generatedAt: Date.now(),
        model: 'claude'
    };
}
