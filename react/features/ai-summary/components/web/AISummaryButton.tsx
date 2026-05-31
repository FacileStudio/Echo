import React from 'react';
import { connect } from 'react-redux';

import { createToolbarEvent } from '../../../analytics/AnalyticsEvents';
import { sendAnalytics } from '../../../analytics/functions';
import { IReduxState } from '../../../app/types';
import { translate } from '../../../base/i18n/functions';
import { IconAI } from '../../../base/icons/svg';
import AbstractButton, { IProps as AbstractButtonProps } from '../../../base/toolbox/components/AbstractButton';
import { closeOverflowMenuIfOpen } from '../../../toolbox/actions.web';
import { toggleAISummaryPanel } from '../../actions';
import { isAISummaryEnabled, isAISummaryPanelOpen } from '../../functions';

interface IProps extends AbstractButtonProps {
    _isPanelOpen: boolean;
}

class AISummaryButton extends AbstractButton<IProps> {
    override accessibilityLabel = 'toolbar.accessibilityLabel.aiSummary';
    override toggledAccessibilityLabel = 'toolbar.accessibilityLabel.closeAiSummary';
    override icon = IconAI;
    override label = 'toolbar.aiSummary';
    override toggledLabel = 'toolbar.closeAiSummary';
    override tooltip = 'toolbar.aiSummary';
    override toggledTooltip = 'toolbar.closeAiSummary';

    override _isToggled() {
        return this.props._isPanelOpen;
    }

    override _handleClick() {
        const { dispatch, _isPanelOpen } = this.props;

        sendAnalytics(createToolbarEvent('toggle.ai-summary', { enable: !_isPanelOpen }));
        dispatch(closeOverflowMenuIfOpen());
        dispatch(toggleAISummaryPanel());
    }
}

const mapStateToProps = (state: IReduxState) => ({
    _isPanelOpen: isAISummaryPanelOpen(state),
    visible: isAISummaryEnabled(state)
});

export default translate(connect(mapStateToProps)(AISummaryButton));
