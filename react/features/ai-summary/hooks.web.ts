import { useSelector } from 'react-redux';

import AISummaryButton from './components/web/AISummaryButton';
import { isAISummaryEnabled } from './functions';

const aiSummary = {
    key: 'ai-summary',
    Content: AISummaryButton,
    group: 2
};

export function useAISummaryButton() {
    const enabled = useSelector(isAISummaryEnabled);

    if (!enabled) {
        return undefined;
    }

    return aiSummary;
}
