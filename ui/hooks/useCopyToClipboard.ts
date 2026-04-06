"use client";

import { useCallback, useRef, useState } from "react";
import { toast } from "sonner";

interface UseCopyToClipboardOptions {
	successMessage?: string;
	errorMessage?: string;
	resetDelay?: number;
}

export function useCopyToClipboard(options: UseCopyToClipboardOptions = {}) {
	const { successMessage = "Copied to clipboard", errorMessage = "Failed to copy", resetDelay = 2000 } = options;
	const [copied, setCopied] = useState(false);
	const timeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

	const fallbackCopy = useCallback((text: string) => {
		if (typeof document === "undefined") {
			return false;
		}
		const textArea = document.createElement("textarea");
		textArea.value = text;
		textArea.setAttribute("readonly", "");
		textArea.style.position = "fixed";
		textArea.style.top = "-9999px";
		textArea.style.left = "-9999px";
		document.body.appendChild(textArea);
		textArea.focus();
		textArea.select();
		textArea.setSelectionRange(0, text.length);
		try {
			return document.execCommand("copy");
		} finally {
			document.body.removeChild(textArea);
		}
	}, []);

	const copy = useCallback(
		async (text: string) => {
			try {
				if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
					await navigator.clipboard.writeText(text);
				} else if (!fallbackCopy(text)) {
					throw new Error("clipboard unavailable");
				}
				setCopied(true);
				toast.success(successMessage);

				if (timeoutRef.current) clearTimeout(timeoutRef.current);
				timeoutRef.current = setTimeout(() => setCopied(false), resetDelay);
			} catch {
				if (fallbackCopy(text)) {
					setCopied(true);
					toast.success(successMessage);
					if (timeoutRef.current) clearTimeout(timeoutRef.current);
					timeoutRef.current = setTimeout(() => setCopied(false), resetDelay);
					return;
				}
				toast.error(errorMessage);
			}
		},
		[fallbackCopy, successMessage, errorMessage, resetDelay],
	);

	return { copy, copied };
}
