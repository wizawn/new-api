/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { lazy, Suspense, useContext, useMemo } from 'react';
import { Route, Routes, useLocation, useParams } from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import { AuthRedirect, PrivateRoute, AdminRoute } from './helpers';
import RegisterForm from './components/auth/RegisterForm';
import LoginForm from './components/auth/LoginForm';
import NotFound from './pages/NotFound';
import Forbidden from './pages/Forbidden';
import { StatusContext } from './context/Status';

import PasswordResetForm from './components/auth/PasswordResetForm';
import PasswordResetConfirm from './components/auth/PasswordResetConfirm';
import OAuth2Callback from './components/auth/OAuth2Callback';
import SetupCheck from './components/layout/SetupCheck';

const Home = lazy(() => import('./pages/Home'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const About = lazy(() => import('./pages/About'));
const Docs = lazy(() => import('./pages/Docs'));
const DocsSetup = lazy(() => import('./pages/DocsSetup'));
const Setup = lazy(() => import('./pages/Setup'));
const User = lazy(() => import('./pages/User'));
const Setting = lazy(() => import('./pages/Setting'));
const Channel = lazy(() => import('./pages/Channel'));
const Token = lazy(() => import('./pages/Token'));
const Redemption = lazy(() => import('./pages/Redemption'));
const TopUp = lazy(() => import('./pages/TopUp'));
const Log = lazy(() => import('./pages/Log'));
const Chat = lazy(() => import('./pages/Chat'));
const Chat2Link = lazy(() => import('./pages/Chat2Link'));
const Midjourney = lazy(() => import('./pages/Midjourney'));
const Pricing = lazy(() => import('./pages/Pricing'));
const Task = lazy(() => import('./pages/Task'));
const ModelPage = lazy(() => import('./pages/Model'));
const ModelDeploymentPage = lazy(() => import('./pages/ModelDeployment'));
const ChatMockPage = lazy(() => import('./pages/ChatMock'));
const TeamAutomationPage = lazy(() => import('./pages/TeamAutomation'));
const Playground = lazy(() => import('./pages/Playground'));
const Subscription = lazy(() => import('./pages/Subscription'));
const PersonalSetting = lazy(() => import('./components/settings/PersonalSetting'));
const UserAgreement = lazy(() => import('./pages/UserAgreement'));
const PrivacyPolicy = lazy(() => import('./pages/PrivacyPolicy'));

function DynamicOAuth2Callback() {
  const { provider } = useParams();
  return <OAuth2Callback type={provider} />;
}

function App() {
  const location = useLocation();
  const [statusState] = useContext(StatusContext);

  const renderWithLoading = (element) => (
    <Suspense fallback={<Loading></Loading>} key={location.pathname}>
      {element}
    </Suspense>
  );

  // 获取模型广场权限配置
  const pricingRequireAuth = useMemo(() => {
    const headerNavModulesConfig = statusState?.status?.HeaderNavModules;
    if (headerNavModulesConfig) {
      try {
        const modules = JSON.parse(headerNavModulesConfig);

        // 处理向后兼容性：如果pricing是boolean，默认不需要登录
        if (typeof modules.pricing === 'boolean') {
          return false; // 默认不需要登录鉴权
        }

        // 如果是对象格式，使用requireAuth配置
        return modules.pricing?.requireAuth === true;
      } catch (error) {
        console.error('解析顶栏模块配置失败:', error);
        return false; // 默认不需要登录
      }
    }
    return false; // 默认不需要登录
  }, [statusState?.status?.HeaderNavModules]);

  return (
    <SetupCheck>
      <Routes>
        <Route
          path='/'
          element={renderWithLoading(<Home />)}
        />
        <Route
          path='/setup'
          element={renderWithLoading(<Setup />)}
        />
        <Route
          path='/docs'
          element={renderWithLoading(<Docs />)}
        />
        <Route
          path='/docs/setup'
          element={renderWithLoading(<DocsSetup />)}
        />
        <Route path='/forbidden' element={<Forbidden />} />
        <Route
          path='/console/models'
          element={
            <AdminRoute>
              {renderWithLoading(<ModelPage />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/deployment'
          element={
            <AdminRoute>
              {renderWithLoading(<ModelDeploymentPage />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/chatmock'
          element={
            <AdminRoute>
              {renderWithLoading(<ChatMockPage />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/team-automation'
          element={
            <AdminRoute>
              {renderWithLoading(<TeamAutomationPage />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/subscription'
          element={
            <AdminRoute>
              {renderWithLoading(<Subscription />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/channel'
          element={
            <AdminRoute>
              {renderWithLoading(<Channel />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/token'
          element={
            <PrivateRoute>
              {renderWithLoading(<Token />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/playground'
          element={
            <PrivateRoute>
              {renderWithLoading(<Playground />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/redemption'
          element={
            <AdminRoute>
              {renderWithLoading(<Redemption />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/user'
          element={
            <AdminRoute>
              {renderWithLoading(<User />)}
            </AdminRoute>
          }
        />
        <Route
          path='/user/reset'
          element={renderWithLoading(<PasswordResetConfirm />)}
        />
        <Route
          path='/login'
          element={
            renderWithLoading(
              <AuthRedirect>
                <LoginForm />
              </AuthRedirect>
            )
          }
        />
        <Route
          path='/register'
          element={
            renderWithLoading(
              <AuthRedirect>
                <RegisterForm />
              </AuthRedirect>
            )
          }
        />
        <Route
          path='/reset'
          element={renderWithLoading(<PasswordResetForm />)}
        />
        <Route
          path='/oauth/github'
          element={renderWithLoading(<OAuth2Callback type='github'></OAuth2Callback>)}
        />
        <Route
          path='/oauth/discord'
          element={renderWithLoading(<OAuth2Callback type='discord'></OAuth2Callback>)}
        />
        <Route
          path='/oauth/oidc'
          element={renderWithLoading(<OAuth2Callback type='oidc'></OAuth2Callback>)}
        />
        <Route
          path='/oauth/linuxdo'
          element={renderWithLoading(<OAuth2Callback type='linuxdo'></OAuth2Callback>)}
        />
        <Route
          path='/oauth/:provider'
          element={renderWithLoading(<DynamicOAuth2Callback />)}
        />
        <Route
          path='/console/setting'
          element={
            <AdminRoute>
              {renderWithLoading(<Setting />)}
            </AdminRoute>
          }
        />
        <Route
          path='/console/personal'
          element={
            <PrivateRoute>
              {renderWithLoading(<PersonalSetting />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/topup'
          element={
            <PrivateRoute>
              {renderWithLoading(<TopUp />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/log'
          element={
            <PrivateRoute>
              {renderWithLoading(<Log />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console'
          element={
            <PrivateRoute>
              {renderWithLoading(<Dashboard />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/midjourney'
          element={
            <PrivateRoute>
              {renderWithLoading(<Midjourney />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/console/task'
          element={
            <PrivateRoute>
              {renderWithLoading(<Task />)}
            </PrivateRoute>
          }
        />
        <Route
          path='/pricing'
          element={
            pricingRequireAuth ? (
              <PrivateRoute>
                {renderWithLoading(<Pricing />)}
              </PrivateRoute>
            ) : (
              renderWithLoading(<Pricing />)
            )
          }
        />
        <Route
          path='/about'
          element={renderWithLoading(<About />)}
        />
        <Route
          path='/user-agreement'
          element={renderWithLoading(<UserAgreement />)}
        />
        <Route
          path='/privacy-policy'
          element={renderWithLoading(<PrivacyPolicy />)}
        />
        <Route
          path='/console/chat/:id?'
          element={renderWithLoading(<Chat />)}
        />
        {/* 方便使用chat2link直接跳转聊天... */}
        <Route
          path='/chat2link'
          element={
            <PrivateRoute>
              {renderWithLoading(<Chat2Link />)}
            </PrivateRoute>
          }
        />
        <Route path='*' element={<NotFound />} />
      </Routes>
    </SetupCheck>
  );
}

export default App;
