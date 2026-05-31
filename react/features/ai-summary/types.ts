export interface ITranscriptEntry {
    language: string;
    participantId: string;
    speaker: string;
    text: string;
    timestamp: number;
}

export interface IParticipantSummary {
    displayName: string;
    participantId: string;
    role: string;
    totalSpeakingTime: number;
}

export interface IMeetingTranscript {
    entries: ITranscriptEntry[];
    meetingId: string;
    participants: Map<string, IParticipantSummary>;
    roomName: string;
    startTime: number;
}

export interface IActionItem {
    assignee: string | null;
    deadline: string | null;
    description: string;
}

export interface ISummaryResult {
    actionItems: IActionItem[];
    decisions: string[];
    generatedAt: number;
    model: string;
    perSpeaker: Record<string, string>;
    summary: string;
    topics: string[];
}

export type SummaryStatus = 'idle' | 'collecting' | 'generating' | 'ready' | 'error';
