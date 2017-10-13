/* eslint-disable no-undef*/
import m from 'mithril';
import prop from 'mithril/stream';

export class Job {
  constructor() {
    this.apps = prop({});
    this.error = prop('');

    const wsScheme = (window.location.protocol === 'https:') ? 'wss://' : 'ws://';
    this.socket = new WebSocket(`${wsScheme}${window.location.host}/ws`);
    // this.socket.onopen = this.init.bind(this);
    this.socket.onmessage = this.receivedUpdate.bind(this);
    this.socket.onerror = this.receivedError.bind(this);
    window.addEventListener('unload', this.disconnect.bind(this));
  }

  disconnect() {
    this.socket.send({ message: 'disconnect' });
  }

  // init() {
  //   this.apps = prop({});
  //   this.error = prop('');
  //
  //   // Need to send request to get current state on servers
  //   const apps = this.apps()[stats.app] || {};
  //   const totals = apps.totals || {};
  //   const jobs = apps.jobs || {};
  //   return {
  //     totals: {
  //       ...totals,
  //       active_jobs: stats.active_jobs || 0,
  //       queued_jobs: (stats.queued_jobs || 0) + (totals.queued_jobs || 0),
  //       requeued_jobs: (stats.requeued_jobs || 0) + (totals.requeued_jobs || 0),
  //       failed_jobs: (stats.failed_jobs || 0) + (totals.failed_jobs || 0),
  //       processed_jobs: (stats.processed_jobs || 0) + (totals.processed_jobs || 0),
  //     },
  //     jobs: {
  //       ...jobs,
  //       ...stats.jobs,
  //     },
  //   };
  //   m.redraw();
  // }

  static initialAppTotals(totals) {
    return totals || {
      active_jobs: 0,
      queued_jobs: 0,
      processed_jobs: 0,
      failed_jobs: 0,
      requeued_jobs: 0,
    };
  }

  updateApps(stats) {
    const app = this.apps()[stats.app] || {};
    const totals = Job.initialAppTotals(app.totals);
    const jobs = app.jobs || {};

    if (stats.job) {
      const job = stats.job;
      job.queue_id = stats.queue_id;

      return {
        totals: {
          ...totals,
          [`${job.status}_jobs`]: (totals[`${job.status}_jobs`] || 0) + 1,
        },
        jobs: {
          ...jobs,
          [job.id]: job,
        },
      };
    }

    const { active_jobs } = stats;

    return {
      jobs,
      totals: {
        ...totals,
        active_jobs: active_jobs || 0, //eslint-disable-line
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
  }
}

export default new Job();
