import m from 'mithril';
import Layout from './components/layout';
import './app.html';

m.route.prefix('');

m.route(document.body, '/', // eslint-disable-line
  {
    '/': { view: () => m(Layout, m('h1', 'Rift Monitoring')) },
  },
);
