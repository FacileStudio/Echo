import {
    AI_SUMMARY_ADD_ENTRY,
    AI_SUMMARY_RESET,
    AI_SUMMARY_SET_ERROR,
    AI_SUMMARY_SET_RESULT,
    AI_SUMMARY_SET_STATUS,
    AI_SUMMARY_TOGGLE_PANEL,
    AI_SUMMARY_UPDATE_PARTICIPANT
} from './actionTypes';
import { IParticipantSummary, ISummaryResult, ITranscriptEntry, SummaryStatus } from './types';

export function addTranscriptEntry(entry: ITranscriptEntry) {
    return {
        type: AI_SUMMARY_ADD_ENTRY,
        entry
    };
}

export function setAISummaryStatus(status: SummaryStatus) {
    return {
        type: AI_SUMMARY_SET_STATUS,
        status
    };
}

export function setAISummaryResult(result: ISummaryResult) {
    return {
        type: AI_SUMMARY_SET_RESULT,
        result
    };
}

export function resetAISummary() {
    return {
        type: AI_SUMMARY_RESET
    };
}

export function toggleAISummaryPanel() {
    return {
        type: AI_SUMMARY_TOGGLE_PANEL
    };
}

export function setAISummaryError(error: string) {
    return {
        type: AI_SUMMARY_SET_ERROR,
        error
    };
}

export function updateAISummaryParticipant(participantId: string, data: IParticipantSummary) {
    return {
        type: AI_SUMMARY_UPDATE_PARTICIPANT,
        participantId,
        data
    };
}
