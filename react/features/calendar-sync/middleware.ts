import { IStore } from '../app/types';
import { SET_CONFIG } from '../base/config/actionTypes';
import { ADD_KNOWN_DOMAINS } from '../base/known-domains/actionTypes';
import MiddlewareRegistry from '../base/redux/MiddlewareRegistry';
import { equals } from '../base/redux/functions';


import { REFRESH_CALENDAR } from './actionTypes';

import { _fetchCalendarEntries, isCalendarEnabled } from './functions';

MiddlewareRegistry.register(store => next => action => {
    const { getState } = store;

    if (!isCalendarEnabled(getState)) {
        return next(action);
    }

    switch (action.type) {
    case ADD_KNOWN_DOMAINS: {
        // XXX Fetch new calendar entries only when an actual domain has
        // become known.
        const oldValue = getState()['features/base/known-domains'];
        const result = next(action);
        const newValue = getState()['features/base/known-domains'];

        equals(oldValue, newValue)
            || _fetchCalendarEntries(store, false, false);

        return result;
    }

    case SET_CONFIG: {
        const result = next(action);

        _fetchCalendarEntries(store, false, false);

        return result;
    }

    case REFRESH_CALENDAR: {
        const result = next(action);

        _fetchCalendarEntries(
            store, action.isInteractive, action.forcePermission);

        return result;
    }
    }

    return next(action);
});

