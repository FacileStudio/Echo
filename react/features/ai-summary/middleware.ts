import { IStore } from '../app/types';
import {
    CONFERENCE_JOINED,
    CONFERENCE_LEFT,
    ENDPOINT_MESSAGE_RECEIVED,
    NON_PARTICIPANT_MESSAGE_RECEIVED
} from '../base/conference/actionTypes';
import { TRANSCRIBER_ID } from '../base/participants/constants';
import { getParticipantById } from '../base/participants/functions';
import MiddlewareRegistry from '../base/redux/MiddlewareRegistry';

import { addTranscriptEntry, resetAISummary, setAISummaryStatus, updateAISummaryParticipant } from './actions';
import { isAISummaryEnabled, shouldAutoSummarize } from './functions';
import logger from './logger';
import { generateSummary } from './summarizer';

const JSON_TYPE_TRANSCRIPTION_RESULT = 'transcription-result';

MiddlewareRegistry.register(store => next => action => {
    switch (action.type) {
    case CONFERENCE_JOINED:
        _onConferenceJoined(store);
        break;

    case CONFERENCE_LEFT:
        _onConferenceLeft(store);
        break;

    case ENDPOINT_MESSAGE_RECEIVED:
    case NON_PARTICIPANT_MESSAGE_RECEIVED:
        _onTranscriptMessage(store, action);
        break;
    }

    return next(action);
});

function _onConferenceJoined(store: IStore) {
    const state = store.getState();

    if (!isAISummaryEnabled(state)) {
        return;
    }

    logger.info('Conference joined — AI summary collector active');
    store.dispatch(resetAISummary());
    store.dispatch(setAISummaryStatus('collecting'));
}

function _onConferenceLeft(store: IStore) {
    const state = store.getState();

    if (!isAISummaryEnabled(state)) {
        return;
    }

    const { entries } = state['features/ai-summary'];

    if (entries.length === 0) {
        logger.info('Conference left with no transcript entries — skipping summary');
        store.dispatch(setAISummaryStatus('idle'));

        return;
    }

    if (shouldAutoSummarize(state)) {
        logger.info(`Conference left with ${entries.length} entries — auto-generating summary`);
        generateSummary(store);
    }
}

function _onTranscriptMessage(store: IStore, action: any) {
    const state = store.getState();

    if (!isAISummaryEnabled(state)) {
        return;
    }

    const { status } = state['features/ai-summary'];

    if (status !== 'collecting') {
        return;
    }

    let json: any;

    if (action.type === ENDPOINT_MESSAGE_RECEIVED) {
        if (!action.participant?.isHidden?.()) {
            return;
        }
        json = action.data;
    } else if (action.type === NON_PARTICIPANT_MESSAGE_RECEIVED && action.id === TRANSCRIBER_ID) {
        json = action.json;
    } else {
        return;
    }

    if (json?.type !== JSON_TYPE_TRANSCRIPTION_RESULT) {
        return;
    }

    if (json.is_interim) {
        return;
    }

    const text = json.transcript?.[0]?.text;

    if (!text?.trim()) {
        return;
    }

    const participantId = json.participant?.id;
    const speakerName = json.participant?.name;
    const participant = participantId ? getParticipantById(state, participantId) : undefined;
    const displayName = participant?.displayName || speakerName || `Speaker ${json.speaker ?? ''}`.trim();

    store.dispatch(addTranscriptEntry({
        speaker: displayName,
        participantId: participantId || '',
        text: text.trim(),
        timestamp: json.timestamp || Date.now(),
        language: json.language || 'en'
    }));

    if (participantId && participant) {
        store.dispatch(updateAISummaryParticipant(participantId, {
            participantId,
            displayName,
            role: participant.role || 'participant',
            totalSpeakingTime: 0
        }));
    }
}
