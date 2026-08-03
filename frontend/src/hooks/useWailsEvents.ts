import { useEffect } from 'react';
import { EventsOn } from '@wailsjs/runtime/runtime';
import type { WailsEvent } from './useSessions';

type EventHandler = (event: WailsEvent) => void;

export function useWailsEvents(onCommand: EventHandler, onState: EventHandler): void {
  useEffect(() => {
    const offCommand = EventsOn('ai:command', (data: unknown) => {
      onCommand(data as WailsEvent);
    });
    const offState = EventsOn('session:state', (data: unknown) => {
      onState(data as WailsEvent);
    });
    return () => {
      offCommand();
      offState();
    };
  }, [onCommand, onState]);
}
