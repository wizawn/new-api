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

import React from 'react';
import { Link } from 'react-router-dom';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  drawerOpen,
  isConsoleRoute,
  onNavigate,
}) => {
  const renderNavLinks = (mobile = false) => {
    const baseClasses =
      'flex-shrink-0 flex items-center gap-1 font-semibold rounded-full transition-all duration-200 ease-in-out';
    const hoverClasses = mobile
      ? 'hover:bg-slate-100 hover:text-slate-950'
      : 'hover:bg-blue-50 hover:text-blue-700';
    const spacingClasses = mobile ? 'px-4 py-3' : 'px-4 py-2.5';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return mainNavLinks.map((link) => {
      const linkContent = <span>{link.text}</span>;

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={commonLinkClasses}
          >
            {linkContent}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link
          key={link.itemKey}
          to={targetPath}
          className={commonLinkClasses}
          onClick={onNavigate}
        >
          {linkContent}
        </Link>
      );
    });
  };

  if (isMobile && !isConsoleRoute) {
    if (!drawerOpen) {
      return null;
    }

    return (
      <div className='mt-4 rounded-[28px] border border-slate-200 bg-white/96 p-3 shadow-[var(--shadow-floating)] backdrop-blur-xl md:hidden'>
        <nav className='flex flex-col gap-1'>{renderNavLinks(true)}</nav>
      </div>
    );
  }

  return (
    <nav className='hidden flex-1 items-center gap-2 overflow-x-auto px-4 whitespace-nowrap scrollbar-hide md:flex'>
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        {renderNavLinks(false)}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
