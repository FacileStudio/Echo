import ReducerRegistry from '../base/redux/ReducerRegistry';

import {
    AI_SUMMARY_ADD_ENTRY,
    AI_SUMMARY_RESET,
    AI_SUMMARY_SET_ERROR,
    AI_SUMMARY_SET_RESULT,
    AI_SUMMARY_SET_STATUS,
    AI_SUMMARY_TOGGLE_PANEL,
    AI_SUMMARY_UPDATE_PARTICIPANT
} from './actionTypes';
import { STORE_NAME } from './constants';
import { IParticipantSummary, ISummaryResult, ITranscriptEntry, SummaryStatus } from './types';

export interface IAISummaryState {
    entries: ITranscriptEntry[];
    error: string | null;
    isPanelOpen: boolean;
    meetingId: string | null;
    participants: { [id: string]: IParticipantSummary };
    result: ISummaryResult | null;
    roomName: string | null;
    startTime: number | null;
    status: SummaryStatus;
}

const DEFAULT_STATE: IAISummaryState = {
    entries: [],
    error: null,
    isPanelOpen: false,
    meetingId: null,
    participants: {},
    result: null,
    roomName: null,
    startTime: null,
    status: 'idle'
};

ReducerRegistry.register<IAISummaryState>(
    STORE_NAME, (state: IAISummaryState = DEFAULT_STATE, action): IAISummaryState => {
        switch (action.type) {
        case AI_SUMMARY_ADD_ENTRY:
            return {
                ...state,
                entries: [ ...state.entries, action.entry ]
            };

        case AI_SUMMARY_SET_STATUS:
            return {
                ...state,
                status: action.status
            };

        case AI_SUMMARY_SET_RESULT:
            return {
                ...state,
                result: action.result,
                status: 'ready'
            };

        case AI_SUMMARY_RESET:
            return { ...DEFAULT_STATE };

        case AI_SUMMARY_TOGGLE_PANEL:
            return {
                ...state,
                isPanelOpen: !state.isPanelOpen
            };

        case AI_SUMMARY_SET_ERROR:
            return {
                ...state,
                error: action.error,
                status: 'error'
            };

        case AI_SUMMARY_UPDATE_PARTICIPANT:
            return {
                ...state,
                participants: {
                    ...state.participants,
                    [action.participantId]: action.data
                }
            };

        default:
            return state;
        }
    }
);
