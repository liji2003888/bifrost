"use client";

import { cn } from "@/lib/utils";

interface BrandMarkProps {
	className?: string;
	compact?: boolean;
}

export function BrandMark({ className, compact = false }: BrandMarkProps) {
	if (compact) {
		return (
			<div
				className={cn(
					"text-foreground flex flex-col items-center justify-center text-center font-semibold tracking-[0.08em]",
					className,
				)}
				aria-label="TCL华星"
			>
				<span className="text-[12px] leading-none">TCL</span>
				<span className="text-[11px] leading-none">华星</span>
			</div>
		);
	}

	return (
		<div className={cn("text-foreground font-semibold tracking-[0.16em]", className)} aria-label="TCL华星">
			TCL华星
		</div>
	);
}
