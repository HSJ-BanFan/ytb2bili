'use client';

import { useEffect } from 'react';

// 过滤浏览器扩展产生的控制台噪音
export function ConsoleFilter() {
  useEffect(() => {
    if (process.env.NODE_ENV !== 'development') return;

    const originalError = console.error;
    const originalWarn = console.warn;

    const filterPatterns = [
      /chrome-extension:\/\//,
      /content_script\.js/,
      /codes\.forEach is not a function/,
      /message channel closed/,
      /ERR_FAILED/,
      /net::ERR_/,
    ];

    const shouldFilter = (args: unknown[]) => {
      const message = args.map(arg => String(arg)).join(' ');
      return filterPatterns.some(pattern => pattern.test(message));
    };

    console.error = (...args: unknown[]) => {
      if (!shouldFilter(args)) {
        originalError.apply(console, args);
      }
    };

    console.warn = (...args: unknown[]) => {
      if (!shouldFilter(args)) {
        originalWarn.apply(console, args);
      }
    };

    // 清理函数：恢复原始 console
    return () => {
      console.error = originalError;
      console.warn = originalWarn;
    };
  }, []);

  return null;
}
