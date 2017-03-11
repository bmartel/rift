import m from 'mithril';
import prop from 'mithril/stream';

export class Stats {
  constructor() {
    this.apps = prop({});
    this.error = prop('');

    const wsScheme = (window.location.protocol === 'https:') ? 'wss://' : 'ws://';
    const ws = new WebSocket(wsScheme + window.location.host + '/ws');
    ws.onopen = this.init.bind(this);
    ws.onmessage = this.receivedUpdate.bind(this);
    ws.onerror = this.receivedError.bind(this);
  }

  init() {
    this.apps = prop({});
    this.error = prop('');

    // Need to send request to get current state on servers
    // const apps = this.apps()[stats.app] || {};
    // const totals = apps.totals || {};
    // const jobs = apps.jobs || {};
    // return {
    //   totals: {
    //     ...totals,
    //     active_jobs: stats.active_jobs || 0,
    //     queued_jobs: (stats.queued_jobs || 0) + (totals.queued_jobs || 0),
    //     requeued_jobs: (stats.requeued_jobs || 0) + (totals.requeued_jobs || 0),
    //     failed_jobs: (stats.failed_jobs || 0) + (totals.failed_jobs || 0),
    //     processed_jobs: (stats.processed_jobs || 0) + (totals.processed_jobs || 0),
    //   },
    //   jobs: {
    //     ...jobs,
    //     ...stats.jobs,
    //   },
    // };
    m.redraw();
  }

  updateApps(stats) {
    const apps = this.apps()[stats.app] || {};
    const totals = apps.totals || {};
    const jobs = apps.jobs || {};
    return {
      totals: {
        ...totals,
        active_jobs: stats.active_jobs || 0,
        queued_jobs: (totals.queued_jobs || 0) + (stats.queued_jobs ? 1 : 0),
        requeued_jobs: (totals.requeued_jobs || 0) + (stats.requeued_jobs ? 1 : 0),
        failed_jobs: (totals.failed_jobs || 0) + (stats.failed_jobs ? 1 : 0),
        processed_jobs: (totals.processed_jobs || 0) + (stats.processed_jobs ? 1 : 0),
      },
      jobs: {
        ...jobs,
        ...stats.jobs,
      },
    };
  }

  receivedUpdate(event) {
    const stats = JSON.parse(event.data);
    this.apps({
      ...this.apps(),
      [stats.app]: this.updateApps(stats),
    });
    m.redraw();
  }

  receivedError(event) {
    this.error(event.data);
    m.redraw();
    console.debug("Error", this.error());
  }
}

export default new Stats();
