import { CSSProperties } from "react";
import logo from '../assets/images/logo.png'

interface ContrailsIconProps {
  className?: string;
  style?: CSSProperties;
}

export function ContrailsIcon({
  className,
  style,
}: ContrailsIconProps) {
  return (
    <img src={logo} alt="Contrails" className={className} style={{ width: '34px', filter: 'invert(1) brightness(1.5)', ...style }} />
  );
}
