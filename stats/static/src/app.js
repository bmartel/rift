import m from 'mithril';
import Layout from './components/layout';
import Dashboard from './components/dashboard';
import './app.css';
import './app.html';

m.route.prefix('');

m.route(document.body, '/', // eslint-disable-line
  {
    '/': { view: () => m(Layout, m(Dashboard)) },
  },
);
